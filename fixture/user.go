package fixture

// go:generate athena_schema -type=User,Post,Comment
type User struct {
	UserID int64  `json:"user_id"`
	Name   string `json:"name"`
}

type Comment struct {
	text string
}

type Post struct {
	Author   User      `test:"" json:"author_info"`
	Comments []Comment `json:"comments"`
}
