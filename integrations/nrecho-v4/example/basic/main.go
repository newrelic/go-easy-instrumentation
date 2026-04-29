package main

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

var db = make(map[string]string)

func ping(c echo.Context) error {
	return c.String(http.StatusOK, "pong")
}

func setupRouter() *echo.Echo {
	e := echo.New()

	e.GET("/ping", ping)
	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "hello")
	})

	// Get user value
	e.GET("/user/:name", func(c echo.Context) error {
		user := c.Param("name")
		value, ok := db[user]
		if ok {
			return c.JSON(http.StatusOK, map[string]string{"user": user, "value": value})
		}
		return c.JSON(http.StatusOK, map[string]string{"user": user, "status": "no value"})
	})

	return e
}

func main() {
	e := setupRouter()
	e.Start(":8080")
}
