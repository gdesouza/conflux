package commands

import (
	"conflux/internal/confluence"
	"conflux/pkg/logger"
)

// newConfluenceClient is a package-level variable to allow test injection of a mock.
// Production code uses the real client constructor; tests can override this.
var newConfluenceClient = func(baseURL, user, token string, log *logger.Logger) confluence.ConfluenceClient {
	return confluence.NewClient(baseURL, user, token, log)
}
