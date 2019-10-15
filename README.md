# athena-schema

Generate an athena (presto) table definition from a golang strcut.

## Install

```
go get github.com/uzimith/athena_schema/cmd/athena_schema
```

## Usage

```
cd fixture
athena_schema -type=User,Post
```

This application need a template file in order to run this command.

```
type Post struct {
	Author    User      `test:"" json:"author_info"` # name priority: json tag > camel case field name
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt string    `athena:"timestamp"` # type overwrite by athena struct tag
}
```
