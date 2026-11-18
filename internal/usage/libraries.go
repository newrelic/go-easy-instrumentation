package usage

import (
	"sort"
	"strings"
)

const agentIntegrations = "github.com/newrelic/go-agent/v3/integrations/"

// library is a third-party or standard library package this tool looks for in
// the application being instrumented.
//
// Only names from this list are ever reported. Import paths found in the
// application are matched against it and then discarded, so an application's
// own packages, private modules, and anything unlisted never leave the machine.
type library struct {
	// name is the reported identifier segment. It must not contain "/".
	name string

	// importPath is the import path, or path prefix, that marks the library as
	// used. Versioned paths are listed separately so that, for example, echo v3
	// and echo v4 are told apart by longest match.
	importPath string

	// integration is the import path this tool adds when it instruments the
	// library. It is empty when this tool has no integration for the library.
	integration string

	// unattributable marks a library this tool supports but whose
	// instrumentation adds only the base agent import, so whether it was applied
	// cannot be observed.
	unattributable bool
}

// knownLibraries is every library reported. Supported libraries come first,
// then popular libraries this tool does not yet instrument. The second group is
// what tells us which integration to build next.
var knownLibraries = []library{
	// Supported by an integration in this tool.
	{name: "gin", importPath: "github.com/gin-gonic/gin", integration: agentIntegrations + "nrgin"},
	{name: "echo-v4", importPath: "github.com/labstack/echo/v4", integration: agentIntegrations + "nrecho-v4"},
	{name: "echo-v3", importPath: "github.com/labstack/echo", integration: agentIntegrations + "nrecho-v3"},
	{name: "chi-v5", importPath: "github.com/go-chi/chi/v5", integration: agentIntegrations + "nrgochi"},
	{name: "grpc", importPath: "google.golang.org/grpc", integration: agentIntegrations + "nrgrpc"},
	{name: "logrus", importPath: "github.com/sirupsen/logrus", integration: agentIntegrations + "logcontext-v2/nrlogrus"},
	{name: "slog", importPath: "log/slog", integration: agentIntegrations + "logcontext-v2/nrslog"},
	{name: "pq", importPath: "github.com/lib/pq", integration: agentIntegrations + "nrpq"},
	{name: "pgx-v5", importPath: "github.com/jackc/pgx/v5", integration: agentIntegrations + "nrpgx5"},
	{name: "nethttp", importPath: "net/http", unattributable: true},

	// Not yet supported by this tool.
	//
	// mysql is listed here deliberately: integrations/nrmysql exists, but its
	// tracing function is not registered in cmd, so the tool never applies it.
	// Move it to the supported group, marked unattributable, once it is.
	{name: "mysql", importPath: "github.com/go-sql-driver/mysql"},
	{name: "sqlite3", importPath: "github.com/mattn/go-sqlite3"},
	{name: "gorm", importPath: "gorm.io/gorm"},
	{name: "sqlx", importPath: "github.com/jmoiron/sqlx"},
	{name: "pgx-v4", importPath: "github.com/jackc/pgx/v4"},
	{name: "mongo", importPath: "go.mongodb.org/mongo-driver"},
	{name: "go-redis-v9", importPath: "github.com/redis/go-redis/v9"},
	{name: "go-redis-v8", importPath: "github.com/go-redis/redis/v8"},
	{name: "redigo", importPath: "github.com/gomodule/redigo"},
	{name: "elasticsearch", importPath: "github.com/elastic/go-elasticsearch"},
	// Later major versions of supported libraries need their own entries:
	// without one, longest-prefix matching would file them under the older
	// version and report a pattern-matching gap instead of a missing
	// integration.
	{name: "echo-v5", importPath: "github.com/labstack/echo/v5"},
	{name: "chi-legacy", importPath: "github.com/go-chi/chi"},
	{name: "gorilla-mux", importPath: "github.com/gorilla/mux"},
	{name: "fiber-v2", importPath: "github.com/gofiber/fiber/v2"},
	{name: "httprouter", importPath: "github.com/julienschmidt/httprouter"},
	{name: "fasthttp", importPath: "github.com/valyala/fasthttp"},
	{name: "gqlgen", importPath: "github.com/99designs/gqlgen"},
	{name: "zap", importPath: "go.uber.org/zap"},
	{name: "zerolog", importPath: "github.com/rs/zerolog"},
	{name: "aws-sdk-go-v2", importPath: "github.com/aws/aws-sdk-go-v2"},
	{name: "aws-sdk-go", importPath: "github.com/aws/aws-sdk-go"},
	{name: "aws-lambda", importPath: "github.com/aws/aws-lambda-go"},
	{name: "kafka-go", importPath: "github.com/segmentio/kafka-go"},
	{name: "sarama", importPath: "github.com/IBM/sarama"},
	{name: "sarama-legacy", importPath: "github.com/Shopify/sarama"},
	{name: "confluent-kafka", importPath: "github.com/confluentinc/confluent-kafka-go"},
	{name: "nats", importPath: "github.com/nats-io/nats.go"},
}

// matchLibrary returns the known library an import path belongs to, preferring
// the longest matching import path so that versioned paths win over their
// unversioned parents.
func matchLibrary(path string) (library, bool) {
	var (
		best  library
		found bool
	)

	for _, lib := range knownLibraries {
		if path != lib.importPath && !strings.HasPrefix(path, lib.importPath+"/") {
			continue
		}
		if !found || len(lib.importPath) > len(best.importPath) {
			best, found = lib, true
		}
	}

	return best, found
}

// detectedLibraries returns the known libraries imported directly by the
// application, sorted by name. Anything not in knownLibraries is dropped.
func detectedLibraries(directImports []string) []library {
	seen := map[string]bool{}
	libs := []library{}

	for _, path := range directImports {
		lib, ok := matchLibrary(path)
		if !ok || seen[lib.name] {
			continue
		}
		seen[lib.name] = true
		libs = append(libs, lib)
	}

	sort.Slice(libs, func(i, j int) bool { return libs[i].name < libs[j].name })

	return libs
}

// libraryStatus reports what happened to a detected library:
//
//   - Success: the library is instrumented, either by this run or already,
//     because the application imports the integration itself.
//   - Warning: this tool supports it, but did not instrument it on a run that
//     otherwise succeeded. That is a gap in pattern matching.
//   - Info: this tool has no integration for it. That is a coverage gap.
//   - None: the outcome cannot be observed, because the run failed or the
//     library's instrumentation cannot be attributed.
//
// instrumented holds the import paths added by this run together with the
// application's own direct imports, so an application that was instrumented
// earlier, by this tool or by hand, does not show up as a gap.
func libraryStatus(lib library, instrumented map[string]bool, runFailed bool) string {
	switch {
	case lib.integration == "" && !lib.unattributable:
		return statusInfo
	case lib.unattributable:
		return statusNone
	case instrumented[lib.integration]:
		return statusSuccess
	case runFailed:
		return statusNone
	default:
		return statusWarning
	}
}
