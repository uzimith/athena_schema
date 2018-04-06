package fixture

import "time"

// go:generate athena_schema -type=User,Post,Comment
type User struct {
	UserID int64  `json:"user_id"`
	Name   string `json:"name"`
}

type Comment struct {
	text   string
	Author User `test:"" json:"author_info"`
}

type Post struct {
	Author    User      `test:"" json:"author_info"`
	Comments  []Comment `json:"comments"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt string    `athena:"timestamp"`
}
