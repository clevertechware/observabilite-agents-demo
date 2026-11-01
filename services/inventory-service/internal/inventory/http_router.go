package inventory

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

func NewHttpRouter(reservations *Reservations, tracing ...otelgin.Option) *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(otelgin.Middleware("inventory-service", append(tracing, otelgin.WithFilter(notHealthz))...))

	restHandlers := &httpHandlers{reservations: reservations}
	router.GET("/healthz", func(c *gin.Context) { c.Status(http.StatusOK) })
	router.GET("/stock/:sku", restHandlers.getStock)
	router.POST("/reservations", restHandlers.reserve)
	return router
}

func notHealthz(r *http.Request) bool {
	return r.URL.Path != "/healthz"
}
