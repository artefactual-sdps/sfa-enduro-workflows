package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/enums"
)

type DIP struct {
	ent.Schema
}

func (DIP) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "dip"},
	}
}

func (DIP) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("uuid", uuid.UUID{}).
			Unique().
			Immutable(),
		field.String("doc_key").
			Immutable().
			Annotations(entsql.Annotation{Size: 1024}),
		field.Enum("status").
			GoType(enums.DIPStatusQueued),
		field.Text("error_message").
			Optional(),
		field.Time("created_at").
			Immutable().
			Default(time.Now),
		field.Time("started_at").
			Optional(),
		field.Time("completed_at").
			Optional(),
		field.String("object_key").
			Annotations(entsql.Annotation{Size: 1024}).
			Optional(),
	}
}
