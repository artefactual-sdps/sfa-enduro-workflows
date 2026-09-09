package workflows_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	temporalsdk_activity "go.temporal.io/sdk/activity"
	temporalsdk_temporal "go.temporal.io/sdk/temporal"
	temporalsdk_testsuite "go.temporal.io/sdk/testsuite"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/activities"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/datatypes"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/enums"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/workflows"
)

type CreateDIPTestSuite struct {
	suite.Suite
	temporalsdk_testsuite.WorkflowTestSuite

	env      *temporalsdk_testsuite.TestWorkflowEnvironment
	workflow *workflows.CreateDIP
	dip      datatypes.DIP
}

var createDIPTestTime = time.Date(2024, 6, 6, 15, 8, 39, 0, time.UTC)

func (s *CreateDIPTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	s.env.SetStartTime(createDIPTestTime)
	s.env.RegisterActivityWithOptions(
		activities.NewUpdateDIP(nil).Execute,
		temporalsdk_activity.RegisterOptions{Name: activities.UpdateDIPName},
	)
	s.workflow = workflows.NewCreateDIP()
	s.dip = datatypes.DIP{
		DBID:      1,
		UUID:      uuid.MustParse("9390594f-84c2-457d-bd6a-618f21f7c954"),
		DocKey:    "CH-000001",
		Status:    enums.DIPStatusQueued,
		CreatedAt: createDIPTestTime.Add(-time.Second),
	}
}

func TestCreateDIP(t *testing.T) {
	suite.Run(t, new(CreateDIPTestSuite))
}

func (s *CreateDIPTestSuite) TestSuccess() {
	wDIP := s.dip
	wDIP.Status = enums.DIPStatusInProgress
	wDIP.StartedAt = createDIPTestTime
	s.env.OnActivity(
		activities.UpdateDIPName,
		mock.AnythingOfType("*context.timerCtx"),
		&activities.UpdateDIPParams{DIP: wDIP},
	).Return(&activities.UpdateDIPResult{}, nil).Once()

	wDIP.ObjectKey = "DIP_9390594f-84c2-457d-bd6a-618f21f7c954.zip"
	wDIP.Status = enums.DIPStatusDone
	wDIP.CompletedAt = createDIPTestTime
	s.env.OnActivity(
		activities.UpdateDIPName,
		mock.AnythingOfType("*context.timerCtx"),
		&activities.UpdateDIPParams{DIP: wDIP},
	).Return(&activities.UpdateDIPResult{}, nil).Once()

	s.env.ExecuteWorkflow(s.workflow.Execute, &workflows.CreateDIPParams{DIP: s.dip})

	s.True(s.env.IsWorkflowCompleted())
	s.env.AssertExpectations(s.T())
	s.NoError(s.env.GetWorkflowError())

	var result workflows.CreateDIPResult
	s.Require().NoError(s.env.GetWorkflowResult(&result))
	s.Equal(workflows.CreateDIPResult{DIP: wDIP}, result)
}

func (s *CreateDIPTestSuite) TestBothUpdatesFail() {
	wDIP := s.dip
	wDIP.Status = enums.DIPStatusInProgress
	wDIP.StartedAt = createDIPTestTime
	s.env.OnActivity(
		activities.UpdateDIPName,
		mock.AnythingOfType("*context.timerCtx"),
		&activities.UpdateDIPParams{DIP: wDIP},
	).Return(nil, errors.New("initial update failed")).Times(3)

	wDIP.Status = enums.DIPStatusFailed
	wDIP.CompletedAt = createDIPTestTime.Add(2 * time.Second)
	wDIP.ErrorMessage = "DIP persistence update failed."
	s.env.OnActivity(
		activities.UpdateDIPName,
		mock.AnythingOfType("*context.timerCtx"),
		&activities.UpdateDIPParams{DIP: wDIP},
	).Return(nil, errors.New("final update failed")).Times(3)

	s.env.ExecuteWorkflow(s.workflow.Execute, &workflows.CreateDIPParams{DIP: s.dip})

	s.True(s.env.IsWorkflowCompleted())
	s.env.AssertExpectations(s.T())
	err := s.env.GetWorkflowError()
	s.ErrorContains(err, "initial update failed")
	s.ErrorContains(err, "final update failed")
}

func (s *CreateDIPTestSuite) TestCancellationPersistsFinalStatus() {
	wDIP := s.dip
	wDIP.Status = enums.DIPStatusInProgress
	wDIP.StartedAt = createDIPTestTime
	s.env.OnActivity(
		activities.UpdateDIPName,
		mock.AnythingOfType("*context.timerCtx"),
		&activities.UpdateDIPParams{DIP: wDIP},
	).Return(&activities.UpdateDIPResult{}, nil).After(5 * time.Second).Once()

	wDIP.Status = enums.DIPStatusFailed
	wDIP.CompletedAt = createDIPTestTime.Add(time.Second)
	wDIP.ErrorMessage = "DIP persistence update failed."
	s.env.OnActivity(
		activities.UpdateDIPName,
		mock.AnythingOfType("*context.timerCtx"),
		&activities.UpdateDIPParams{DIP: wDIP},
	).Return(&activities.UpdateDIPResult{}, nil).After(2 * time.Second).Once()

	s.env.RegisterDelayedCallback(s.env.CancelWorkflow, time.Second)
	s.env.ExecuteWorkflow(s.workflow.Execute, &workflows.CreateDIPParams{DIP: s.dip})

	s.True(s.env.IsWorkflowCompleted())
	s.env.AssertExpectations(s.T())
	s.True(temporalsdk_temporal.IsCanceledError(s.env.GetWorkflowError()))
	s.Equal(createDIPTestTime.Add(3*time.Second), s.env.Now().UTC())
}
