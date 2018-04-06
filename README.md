# athena-schema

Generate an athena (presto) table definition from a golang strcut.

```
athena_schema -type=User,Post
```

This applicatio need a template file to run this command.

## Usage

```
type Post struct {
	Author    User      `test:"" json:"author_info"` # name priority: json tag > camel case field name
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt string    `athena:"timestamp"` # type overwrite by athena struct tag
}
```

## Task

* map support
* array support
* struct support
