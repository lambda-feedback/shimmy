package lambda

// ProxySource represents the source of a lambda request.
type ProxySource string

const (
	// ProxySourceApiGatewayV1 represents an API Gateway v1 request.
	ProxySourceApiGatewayV1 ProxySource = "API_GW_V1"

	// ProxySourceApiGatewayV2 represents an API Gateway v2 request.
	ProxySourceApiGatewayV2 ProxySource = "API_GW_V2"

	// ProxySourceAlb represents an Application Load Balancer request.
	ProxySourceAlb ProxySource = "ALB"
)

type Config struct {
	// ProxySource is the source of the AWS Lambda event.
	ProxySource ProxySource `conf:"proxy_source"`
}

var DefaultConfig = map[string]any{
	"proxy_source": ProxySourceApiGatewayV2,
}
