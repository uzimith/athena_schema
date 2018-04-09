# athena-schema

Generate an athena (presto) table definition from a golang strcut.

## Install

```
go get github.com/uzimith/athena-schema/cmd/athena-schema
```

## Usage

```
athena_schema -type=User,Post ./fixture/
```

This applicatio need a template file to run this command.

```
type Post struct {
	Author    User      `test:"" json:"author_info"` # name priority: json tag > camel case field name
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt string    `athena:"timestamp"` # type overwrite by athena struct tag
}
```
