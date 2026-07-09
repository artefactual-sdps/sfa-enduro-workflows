package workercmd

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"

	"github.com/artefactual-labs/bagit-gython"
	"github.com/artefactual-sdps/temporal-activities/archiveextract"
	"github.com/artefactual-sdps/temporal-activities/archivezip"
	"github.com/artefactual-sdps/temporal-activities/bagcreate"
	"github.com/artefactual-sdps/temporal-activities/bagvalidate"
	"github.com/artefactual-sdps/temporal-activities/bucketupload"
	"github.com/artefactual-sdps/temporal-activities/ffvalidate"
	"github.com/artefactual-sdps/temporal-activities/removepaths"
	"github.com/artefactual-sdps/temporal-activities/xmlvalidate"
	"github.com/go-logr/logr"
	"github.com/jonboulle/clockwork"
	"go.artefactual.dev/ssclient"
	"go.artefactual.dev/tools/bucket"
	"go.artefactual.dev/tools/clientauth"
	"go.artefactual.dev/tools/temporal"
	temporalsdk_activity "go.temporal.io/sdk/activity"
	temporalsdk_client "go.temporal.io/sdk/client"
	temporalsdk_interceptor "go.temporal.io/sdk/interceptor"
	temporalsdk_worker "go.temporal.io/sdk/worker"
	temporalsdk_workflow "go.temporal.io/sdk/workflow"
	"gocloud.dev/blob"
	_ "gocloud.dev/blob/fileblob"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/activities"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/amss"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/apis"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/cantons"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/config"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/fformat"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/fvalidate"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/workflows"
)

const Name = "sfa-enduro-worker"

type Main struct {
	logger         logr.Logger
	cfg            config.Config
	temporalWorker temporalsdk_worker.Worker
	temporalClient temporalsdk_client.Client
	bucket         *blob.Bucket
	bagValidator   *bagit.Validator
}

func NewMain(logger logr.Logger, cfg config.Config) *Main {
	return &Main{
		logger: logger,
		cfg:    cfg,
	}
}

func (m *Main) Run(ctx context.Context) error {
	c, err := temporalsdk_client.Dial(temporalsdk_client.Options{
		HostPort:  m.cfg.Temporal.Address,
		Namespace: m.cfg.Temporal.Namespace,
		Logger:    temporal.Logger(m.logger.WithName("temporal")),
	})
	if err != nil {
		m.logger.Error(err, "Unable to create Temporal client.")
		return err
	}
	m.temporalClient = c

	w := temporalsdk_worker.New(m.temporalClient, m.cfg.Worker.TaskQueue, temporalsdk_worker.Options{
		EnableSessionWorker:               true,
		MaxConcurrentSessionExecutionSize: m.cfg.Worker.MaxConcurrentSessions,
		Interceptors: []temporalsdk_interceptor.WorkerInterceptor{
			temporal.NewLoggerInterceptor(m.logger),
		},
	})
	m.temporalWorker = w

	veraPDFValidator := fvalidate.NewVeraPDFValidator(m.cfg.Preprocessing.FileValidate.VeraPDF.Path)

	// Set up BagIt validator.
	m.bagValidator, err = bagit.NewValidator(
		bagit.WithCacheDir(m.cfg.Preprocessing.BagValidate.CacheDir),
		bagit.WithPoolSize(m.cfg.Preprocessing.BagValidate.PoolSize),
	)
	if err != nil {
		m.logger.Error(err, "Unable to create BagIt validator.")
		return err
	}

	// Set up APIS client.
	var apisClient apis.Client
	if m.cfg.APIS.Enabled {
		var tokenProvider clientauth.AccessTokenProvider
		if m.cfg.APIS.OIDC.Enabled {
			tokenProvider, err = clientauth.NewOIDCAccessTokenProvider(
				ctx, m.cfg.APIS.OIDC.OIDCAccessTokenProviderConfig,
			)
			if err != nil {
				m.logger.Error(err, "Unable to create OIDC token provider for APIS client.")
				return err
			}
		}
		if apisClient, err = apis.NewClient(m.cfg.APIS, nil, tokenProvider); err != nil {
			m.logger.Error(err, "Unable to create APIS client.")
			return err
		}
	}

	ssClient, err := ssclient.New(m.cfg.Poststorage.AMSS)
	if err != nil {
		return fmt.Errorf("unable to create Archivematica Storage Service client: %v", err)
	}

	if !m.cfg.APIS.Enabled {
		m.bucket, err = bucket.NewWithConfig(ctx, &m.cfg.Poststorage.Cantons.Bucket)
		if err != nil {
			return fmt.Errorf("unable to open Cantons poststorage bucket: %v", err)
		}
	}

	m.registerPreprocessingWorkflow(apisClient, veraPDFValidator)
	m.registerPoststorageWorkflow(ssClient.Packages(), apisClient, m.bucket)

	if err := w.Start(); err != nil {
		m.logger.Error(err, "Worker failed to start.")
		return err
	}

	return nil
}

func (m *Main) Close() error {
	var e error

	if m.temporalWorker != nil {
		m.temporalWorker.Stop()
	}

	if m.temporalClient != nil {
		m.temporalClient.Close()
	}

	if m.bucket != nil {
		if err := m.bucket.Close(); err != nil {
			e = errors.Join(e, fmt.Errorf("couldn't close Cantons poststorage bucket: %v", err))
		}
	}

	if err := m.bagValidator.Close(); err != nil {
		e = errors.Join(e, fmt.Errorf("couldn't close BagIt validator: %v", err))
	}

	return e
}

func (m *Main) registerPreprocessingWorkflow(
	apisClient apis.Client,
	veraPDFValidator fvalidate.Validator,
) {
	m.temporalWorker.RegisterWorkflowWithOptions(
		workflows.NewPreprocessing(m.cfg.Preprocessing, m.cfg.APIS.Enabled).Execute,
		temporalsdk_workflow.RegisterOptions{Name: m.cfg.Preprocessing.WorkflowName},
	)

	m.temporalWorker.RegisterActivityWithOptions(
		archiveextract.New(archiveextract.Config{}).Execute,
		temporalsdk_activity.RegisterOptions{Name: archiveextract.Name},
	)
	m.temporalWorker.RegisterActivityWithOptions(
		bagvalidate.New(m.bagValidator).Execute,
		temporalsdk_activity.RegisterOptions{Name: bagvalidate.Name},
	)
	m.temporalWorker.RegisterActivityWithOptions(
		activities.NewUnbag().Execute,
		temporalsdk_activity.RegisterOptions{Name: activities.UnbagName},
	)
	m.temporalWorker.RegisterActivityWithOptions(
		activities.NewIdentifySIP().Execute,
		temporalsdk_activity.RegisterOptions{Name: activities.IdentifySIPName},
	)
	m.temporalWorker.RegisterActivityWithOptions(
		activities.NewValidateStructure().Execute,
		temporalsdk_activity.RegisterOptions{Name: activities.ValidateStructureName},
	)
	m.temporalWorker.RegisterActivityWithOptions(
		activities.NewValidateSIPName().Execute,
		temporalsdk_activity.RegisterOptions{Name: activities.ValidateSIPNameName},
	)
	m.temporalWorker.RegisterActivityWithOptions(
		activities.NewVerifyManifest().Execute,
		temporalsdk_activity.RegisterOptions{Name: activities.VerifyManifestName},
	)
	m.temporalWorker.RegisterActivityWithOptions(
		ffvalidate.New(m.cfg.Preprocessing.FileFormat).Execute,
		temporalsdk_activity.RegisterOptions{Name: ffvalidate.Name},
	)
	m.temporalWorker.RegisterActivityWithOptions(
		activities.NewValidateFiles(
			fformat.NewSiegfriedEmbed(),
			veraPDFValidator,
		).Execute,
		temporalsdk_activity.RegisterOptions{Name: activities.ValidateFilesName},
	)
	m.temporalWorker.RegisterActivityWithOptions(
		activities.NewAddPREMISObjects(rand.Reader).Execute,
		temporalsdk_activity.RegisterOptions{Name: activities.AddPREMISObjectsName},
	)
	m.temporalWorker.RegisterActivityWithOptions(
		activities.NewAddPREMISEvent().Execute,
		temporalsdk_activity.RegisterOptions{Name: activities.AddPREMISEventName},
	)
	m.temporalWorker.RegisterActivityWithOptions(
		activities.NewAddPREMISAgent().Execute,
		temporalsdk_activity.RegisterOptions{Name: activities.AddPREMISAgentName},
	)
	m.temporalWorker.RegisterActivityWithOptions(
		activities.NewAddPREMISValidationEvent(
			clockwork.NewRealClock(),
			rand.Reader,
			veraPDFValidator,
		).Execute,
		temporalsdk_activity.RegisterOptions{Name: activities.AddPREMISValidationEventName},
	)
	m.temporalWorker.RegisterActivityWithOptions(
		xmlvalidate.New(xmlvalidate.NewXMLLintValidator()).Execute,
		temporalsdk_activity.RegisterOptions{Name: xmlvalidate.Name},
	)
	m.temporalWorker.RegisterActivityWithOptions(
		activities.NewValidatePREMIS(xmlvalidate.NewXMLLintValidator()).Execute,
		temporalsdk_activity.RegisterOptions{Name: activities.ValidatePREMISName},
	)
	m.temporalWorker.RegisterActivityWithOptions(
		activities.NewTransformSIP().Execute,
		temporalsdk_activity.RegisterOptions{Name: activities.TransformSIPName},
	)
	m.temporalWorker.RegisterActivityWithOptions(
		activities.NewWriteIdentifierFile().Execute,
		temporalsdk_activity.RegisterOptions{Name: activities.WriteIdentifierFileName},
	)
	m.temporalWorker.RegisterActivityWithOptions(
		apis.NewCreateImportTaskActivity(apisClient).Execute,
		temporalsdk_activity.RegisterOptions{Name: apis.CreateImportTaskActivityName},
	)
	m.temporalWorker.RegisterActivityWithOptions(
		apis.NewPollImportTaskStatusActivity(apisClient, m.cfg.APIS.PollInterval).Execute,
		temporalsdk_activity.RegisterOptions{Name: apis.PollImportTaskStatusActivityName},
	)
	m.temporalWorker.RegisterActivityWithOptions(
		bagcreate.New(m.cfg.Preprocessing.BagCreate).Execute,
		temporalsdk_activity.RegisterOptions{Name: bagcreate.Name},
	)
}

func (m *Main) registerPoststorageWorkflow(
	packages *ssclient.PackagesService,
	apisClient apis.Client,
	cantonsBucket *blob.Bucket,
) {
	if m.cfg.APIS.Enabled {
		// Register APIS workflow and specific activities.
		m.temporalWorker.RegisterWorkflowWithOptions(
			workflows.NewPoststorageAPIS(m.cfg.Poststorage).Execute,
			temporalsdk_workflow.RegisterOptions{Name: m.cfg.Poststorage.APIS.WorkflowName},
		)
		m.temporalWorker.RegisterActivityWithOptions(
			apis.NewCreateImportRunActivity(apisClient).Execute,
			temporalsdk_activity.RegisterOptions{Name: apis.CreateImportRunActivityName},
		)
		m.temporalWorker.RegisterActivityWithOptions(
			apis.NewPollImportRunStatusActivity(apisClient, m.cfg.APIS.PollInterval).Execute,
			temporalsdk_activity.RegisterOptions{Name: apis.PollImportRunStatusActivityName},
		)
	} else {
		// Register Cantons workflow and specific activities.
		m.temporalWorker.RegisterWorkflowWithOptions(
			workflows.NewPoststorageCantons(m.cfg.Poststorage).Execute,
			temporalsdk_workflow.RegisterOptions{Name: m.cfg.Poststorage.Cantons.WorkflowName},
		)
		m.temporalWorker.RegisterActivityWithOptions(
			cantons.NewParseActivity().Execute,
			temporalsdk_activity.RegisterOptions{Name: cantons.ParseActivityName},
		)
		m.temporalWorker.RegisterActivityWithOptions(
			cantons.NewCombineMDActivity().Execute,
			temporalsdk_activity.RegisterOptions{Name: cantons.CombineMDActivityName},
		)
		m.temporalWorker.RegisterActivityWithOptions(
			archivezip.New().Execute,
			temporalsdk_activity.RegisterOptions{Name: archivezip.Name},
		)
		m.temporalWorker.RegisterActivityWithOptions(
			bucketupload.New(cantonsBucket).Execute,
			temporalsdk_activity.RegisterOptions{Name: bucketupload.Name},
		)
	}

	// Register common activities for both Cantons and APIS workflows.
	m.temporalWorker.RegisterActivityWithOptions(
		amss.NewGetAIPPathActivity(packages).Execute,
		temporalsdk_activity.RegisterOptions{Name: amss.GetAIPPathActivityName},
	)
	m.temporalWorker.RegisterActivityWithOptions(
		amss.NewFetchActivity(packages).Execute,
		temporalsdk_activity.RegisterOptions{Name: amss.FetchActivityName},
	)
	m.temporalWorker.RegisterActivityWithOptions(
		removepaths.New().Execute,
		temporalsdk_activity.RegisterOptions{Name: removepaths.Name},
	)
}
