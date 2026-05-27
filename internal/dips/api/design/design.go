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
	Description("API used to request DIP creation and poll DIP details.")
	Version("0.1.0")
})

var DIPID = Type("DIPID", String, func() {
	Description("DIP identifier.")
	Format(FormatUUID)
	Example("3f38d6f4-7b19-4db8-8d7d-693b84a9a2fb")
})

var DocKey = Type("DocKey", String, func() {
	Description("Document key.")
	MinLength(1)
	Example("CH-000001")
})

var DIPStatus = Type("DIPStatus", String, func() {
	Description("Current DIP status.")
	Enum("queued", "in progress", "done", "failed")
	Example("done")
})

var DateTime = Type("DateTime", String, func() {
	Description("RFC3339 timestamp.")
	Format(FormatDateTime)
	Example("2026-05-27T15:04:05Z")
})

var ObjectKey = Type("ObjectKey", String, func() {
	Description("Object store key for the completed DIP.")
	MinLength(1)
	Example("dips/3f38d6f4-7b19-4db8-8d7d-693b84a9a2fb.zip")
})

var CreateDIPResult = Type("CreateDIPResult", func() {
	Description("DIP creation request result.")
	Field(1, "id", DIPID, "Identifier assigned to the DIP.")
	Required("id")
})

var DIP = Type("DIP", func() {
	Description("DIP details.")
	Field(1, "id", DIPID, "DIP identifier.")
	Field(2, "docKey", DocKey, "Document key used to create the DIP.")
	Field(3, "status", DIPStatus, "Current DIP status.")
	Field(4, "created_at", DateTime, "Time when the DIP was requested.")
	Field(5, "started_at", DateTime, "Time when DIP processing started.")
	Field(6, "completed_at", DateTime, "Time when DIP processing completed.")
	Field(7, "object_key", ObjectKey, "Object store key for the completed DIP.")
	Required("id", "docKey", "status", "created_at")
})

var _ = Service("DIPs", func() {
	Description("Requests DIP creation and reports their details.")
	Security(BearerAuth)

	Error("unauthorized", ErrorResult, "Missing or invalid bearer token.")
	Error("internal_error", ErrorResult, "Unexpected server error.", func() {
		Fault()
	})

	HTTP(func() {
		Response("unauthorized", StatusUnauthorized)
		Response("internal_error", StatusInternalServerError)
	})

	Method("create", func() {
		Description("Request DIP creation for a document key.")
		Payload(func() {
			BearerTokenField(1, "token", String, "OIDC bearer token.")
			Field(2, "docKey", DocKey, "Document key used to create the DIP.")
			Required("token", "docKey")
		})
		Result(CreateDIPResult)
		Error("not_valid", ErrorResult, "Invalid request parameters.")

		HTTP(func() {
			POST("/dips")
			Header("token:Authorization")
			Body(func() {
				Attribute("docKey")
			})
			Response(StatusAccepted)
			Response("not_valid", StatusBadRequest)
		})
	})

	Method("show", func() {
		Description("Get DIP details.")
		Payload(func() {
			BearerTokenField(1, "token", String, "OIDC bearer token.")
			Field(2, "id", DIPID, "DIP identifier.")
			Required("token", "id")
		})
		Result(DIP)
		Error("not_found", ErrorResult, "DIP not found.")

		HTTP(func() {
			GET("/dips/{id}")
			Header("token:Authorization")
			Response(StatusOK)
			Response("not_found", StatusNotFound)
		})
	})
})
