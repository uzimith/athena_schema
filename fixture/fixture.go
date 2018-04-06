package fixture

import (
	"time"
)

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

type HttpLog struct {
	Requestr Request  `json:"request_parameter"`
	Response Response `json:"response,omitempty"`
}

type Request struct {
	Method         string              `json:"method"`
	URL            string              `json:"url"`
	HTTPHeader     map[string][]string `json:"http_header"`
	RequestBody    string              `json:"request_body"`
	QueryParameter string              `json:"query_parameter"`
}

type Response struct {
	StatusCode   int                 `json:"status_code"`
	HTTPHeader   map[string][]string `json:"http_header"`
	ResponseBody string              `json:"response_body"`
}
