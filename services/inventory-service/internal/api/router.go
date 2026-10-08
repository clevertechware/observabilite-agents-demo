package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

func NewRouter(repository StockRepository, pricingClient PriceProvider, tracing ...otelgin.Option) *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(otelgin.Middleware("inventory-service", append(tracing, otelgin.WithFilter(notHealthz))...))

	handlers := &handlers{repository: repository, pricing: pricingClient}
	router.GET("/healthz", func(c *gin.Context) { c.Status(http.StatusOK) })
	router.GET("/stock/:sku", handlers.getStock)
	router.POST("/reservations", handlers.reserve)
	return router
}

func notHealthz(r *http.Request) bool {
	return r.URL.Path != "/healthz"
}
