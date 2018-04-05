package fixture

// go:generate athena_schema -type=User
type User struct {
	UserID int64  `json:"user_id"`
	Name   string `json:"name"`
}
