/*
Package design is the single source of truth of the DIPs API. It uses the Goa
design language (https://goa.design) which is a Go DSL.

We describe the service which maps to resources in REST. The service defines
its own methods, errors, etc...
*/
package design

import (
	. "goa.design/goa/v3/dsl" //nolint:staticcheck
	"goa.design/goa/v3/expr"
	cors "goa.design/plugins/v3/cors/dsl"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/enums"
)

var BearerAuth = BearerSecurity("bearer", func() {
	Description(
		"Authenticate with an access token supplied as Authorization: Bearer [token]. " +
			"Clients obtain tokens from an external OIDC provider.",
	)
})

var (
	OperationTimeout   = Interceptor("OperationTimeout")
	ServerErrorHandler = Interceptor("ServerErrorHandler")
)

var _ = API("DIPs", func() {
	Title("DIPs API")
	Description("The DIPs API is used to request DIP creation and retrieve DIP details.")
	Version("0.2.0")
	Meta("openapi:versions", "2.0", "3.0", "3.2")
	ServerInterceptor(OperationTimeout)
	ServerInterceptor(ServerErrorHandler)
	Randomizer(expr.NewDeterministicRandomizer())
	Server("sfa-dips", func() {
		Services("DIPs")
		Host("localhost", func() {
			URI("http://localhost:8080")
		})
	})
	HTTP(func() {
		Consumes("application/json")
	})
	cors.Origin("$SFA_DIPS_API_CORS_ORIGIN", func() {
		cors.Methods("GET", "HEAD", "POST", "OPTIONS")
		cors.Headers("Authorization", "Content-Type")
	})
})

var DIPID = Type("DIPID", String, func() {
	Description("DIPID uniquely identifies a DIP.")
	Format(FormatUUID)
	Example("3f38d6f4-7b19-4db8-8d7d-693b84a9a2fb")
})

var DocKey = Type("DocKey", String, func() {
	Description("DocKey is the document key used to create a DIP.")
	MinLength(1)
	MaxLength(1024)
	Example("CH-000001")
})

var DIPStatus = Type("DIPStatus", String, func() {
	Description("DIPStatus represents the current status of a DIP.")
	Enum(enums.DIPStatusInterfaces()...)
	Example(enums.DIPStatusDone.String())
})

var DateTime = Type("DateTime", String, func() {
	Description("DateTime represents an RFC3339 timestamp.")
	Format(FormatDateTime)
	Example("2026-05-27T15:04:05Z")
})

var ObjectKey = Type("ObjectKey", String, func() {
	Description("ObjectKey represents the object store key for a DIP.")
	MinLength(1)
	Example("dips/3f38d6f4-7b19-4db8-8d7d-693b84a9a2fb.zip")
})

var _ = Service("DIPs", func() {
	Description("The DIPs service requests DIP creation and retrieves DIP details.")
	Security(BearerAuth)

	Error("bad_request")
	Error("unauthorized")
	Error("not_found")
	Error("internal_error", ErrorResult, func() {
		Fault()
	})

	HTTP(func() {
		Response("internal_error", StatusInternalServerError, goaErrorResponse())
	})

	Method("livez", func() {
		Description(
			"The livez method provides a simple check that the DIPs service is running and able to respond to requests. A successful response indicates that the service is running, but does not guarantee that it is able to process requests successfully.",
		)
		NoSecurity()
		HTTP(func() {
			GET("/livez")
			Response(StatusOK)
		})
	})

	Method("create", func() {
		Description("The create method requests DIP creation for the given document key.")
		Payload(func() {
			BearerToken("token", String, "The token field contains the OIDC bearer token.")
			Attribute("docKey", DocKey, "The docKey field contains the document key used to create the DIP.")
			Attribute("ignoreCache", Boolean, func() {
				Description(
					"The ignoreCache field indicates whether to ignore a cached DIP previously created for this docKey. When ignoreCache is true, a new DIP is created for the given docKey even if a cached DIP exists. When ignoreCache is false or omitted, the ID of a cached DIP may be returned.",
				)
				Default(false)
				Example(true)
			})
			Required("docKey")
		})
		Result(func() {
			Attribute("id", DIPID, "The id field contains the identifier assigned to the DIP.")
			Required("id")
		})

		HTTP(func() {
			POST("/dips")
			Header("token:Authorization")
			Body(func() {
				Attribute("docKey")
				Attribute("ignoreCache")
			})
			Response(StatusAccepted)
			Response("bad_request", StatusBadRequest, goaErrorResponse())
			Response("unauthorized", StatusUnauthorized, goaErrorResponse())
		})
	})

	Method("show", func() {
		Description("The show method retrieves DIP details.")
		Payload(func() {
			BearerToken("token", String, "The token field contains the OIDC bearer token.")
			Attribute("id", DIPID, "The id field contains the DIP identifier.")
			Required("id")
		})
		Result(func() {
			Attribute("id", DIPID, "The id field uniquely identifies the DIP.")
			Attribute("docKey", DocKey, "The docKey field contains the document key used to create the DIP.")
			Attribute("status", DIPStatus, "The status field contains the current DIP status.")
			Attribute("error_message", String, func() {
				Description("The error_message field contains an error message if the DIP failed.")
				Example("ACTAPro returned: 404 Not Found; Document not found")
			})
			Attribute("created_at", DateTime, "The created_at field contains the time when the DIP was requested.")
			Attribute("started_at", DateTime, "The started_at field contains the time when DIP processing started.")
			Attribute(
				"completed_at",
				DateTime,
				"The completed_at field contains the time when DIP processing completed.",
			)
			Attribute(
				"object_key",
				ObjectKey,
				"The object_key field contains the object store key for the completed DIP.",
			)
			Required("id", "docKey", "status", "created_at")
		})

		HTTP(func() {
			GET("/dips/{id}")
			Header("token:Authorization")
			Response(StatusOK)
			Response("bad_request", StatusBadRequest, goaErrorResponse())
			Response("not_found", StatusNotFound, goaErrorResponse())
			Response("unauthorized", StatusUnauthorized, goaErrorResponse())
		})
	})
})

func goaErrorResponse() func() {
	return func() {
		ContentType("application/json")
	}
}
