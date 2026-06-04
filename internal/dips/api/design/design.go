package design

import . "goa.design/goa/v3/dsl" //nolint:staticcheck

var BearerAuth = BearerSecurity("bearer", func() {
	Description(
		"Authenticate with an access token supplied as Authorization: Bearer [token]. " +
			"Clients obtain tokens from an external OIDC provider.",
	)
})

var _ = API("DIPs", func() {
	Title("DIPs API")
	Description("The DIPs API is used to request DIP creation and retrieve DIP details.")
	Version("0.1.0")
})

var DIPID = Type("DIPID", String, func() {
	Description("DIPID uniquely identifies a DIP.")
	Format(FormatUUID)
	Example("3f38d6f4-7b19-4db8-8d7d-693b84a9a2fb")
})

var DocKey = Type("DocKey", String, func() {
	Description("DocKey is the document key used to create a DIP.")
	MinLength(1)
	Example("CH-000001")
})

var DIPStatus = Type("DIPStatus", String, func() {
	Description("DIPStatus represents the current status of a DIP.")
	Enum("queued", "in progress", "done", "failed")
	Example("done")
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
	Error("internal_server_error", func() {
		Fault()
	})

	HTTP(func() {
		Response("bad_request", StatusBadRequest, goaErrorResponse())
		Response("unauthorized", StatusUnauthorized, goaErrorResponse())
		Response("internal_server_error", StatusInternalServerError, goaErrorResponse())
	})

	Method("create", func() {
		Description("The create method requests DIP creation for a document key.")
		Payload(func() {
			BearerTokenField(1, "token", String, "The token field contains the OIDC bearer token.")
			Field(2, "docKey", DocKey, "The docKey field contains the document key used to create the DIP.")
			Required("token", "docKey")
		})
		Result(func() {
			Field(1, "id", DIPID, "The id field contains the identifier assigned to the DIP.")
			Required("id")
		})

		HTTP(func() {
			POST("/dips")
			Header("token:Authorization")
			Body(func() {
				Attribute("docKey")
			})
			Response(StatusAccepted)
		})
	})

	Method("show", func() {
		Description("The show method retrieves DIP details.")
		Payload(func() {
			BearerTokenField(1, "token", String, "The token field contains the OIDC bearer token.")
			Field(2, "id", DIPID, "The id field contains the DIP identifier.")
			Required("token", "id")
		})
		Result(func() {
			Field(1, "id", DIPID, "The id field uniquely identifies the DIP.")
			Field(2, "docKey", DocKey, "The docKey field contains the document key used to create the DIP.")
			Field(3, "status", DIPStatus, "The status field contains the current DIP status.")
			Field(4, "created_at", DateTime, "The created_at field contains the time when the DIP was requested.")
			Field(5, "started_at", DateTime, "The started_at field contains the time when DIP processing started.")
			Field(
				6,
				"completed_at",
				DateTime,
				"The completed_at field contains the time when DIP processing completed.",
			)
			Field(
				7,
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
			Response("not_found", StatusNotFound, goaErrorResponse())
		})
	})
})

func goaErrorResponse() func() {
	return func() {
		ContentType("application/json")
	}
}
