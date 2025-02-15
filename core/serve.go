package core

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/spf13/afero"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"

	pkgHealth "github.com/anz-bank/pkg/health"
	pkg "github.com/anz-bank/pkg/log"
	zero "github.com/anz-bank/pkg/logging"
	"github.com/anz-bank/sysl-go/config"
	"github.com/anz-bank/sysl-go/health"
	"github.com/anz-bank/sysl-go/log"
	"github.com/anz-bank/sysl-go/validator"
)

type serveContextKey int

const (
	serveYAMLConfigFileKey serveContextKey = iota
	defaultContextTimeout                  = 30 * time.Second
)

type ErrDisplayHelp int

func (e ErrDisplayHelp) Error() string {
	return "Display help"
}

// WithConfigFile adds configuration data into the context. This will be
// used as the source of application configuration data, instead of the
// default behaviour of reading configuration from the config file path
// specified by command line arguments. Data must be in YAML format.
func WithConfigFile(ctx context.Context, yamlConfigData []byte) context.Context {
	return context.WithValue(ctx, serveYAMLConfigFileKey, yamlConfigData)
}

// Serve is deprecated and will be removed once downstream applications cease
// depending upon it. Generated code will no longer call this function.
// This is a shim for compatibility with code generated by sysl-go versions v0.122.0 & earlier.
func Serve(
	ctx context.Context,
	downstreamConfig, createService, serviceInterface interface{},
	newManagers func(ctx context.Context, serviceIntf interface{}, hooks *Hooks) (Manager, *GrpcServerManager, error),
) error {
	srv, err := NewServer(ctx, downstreamConfig, createService, serviceInterface, newManagers)
	if err != nil {
		return err
	}
	return srv.Start()
}

func validateConfig(ctx context.Context, hooks *Hooks, conf *config.DefaultConfig) error {
	// Validate the hooks returned from service creation.
	if hooks != nil && hooks.ValidateConfig != nil {
		if err := hooks.ValidateConfig(ctx, conf); err != nil {
			return err
		}
	}
	return validator.Validate(conf)
}

func withLogLevel(ctx context.Context, defaultConfig *config.DefaultConfig) context.Context {
	// Set the level against the logger. The level will be either the value found within the
	// configuration or the default value (info).
	level := log.InfoLevel
	if defaultConfig.Library.Log.Level != 0 {
		level = defaultConfig.Library.Log.Level
	}
	return log.WithLevel(ctx, level)
}

// NewTemporalWorker creates a Temporal Worker that implements StoppableServer. This is meant to be
// called by a generated temporal worker.
//
// `ctx` is a context created by user.
// `taskQueueName` is a generated task queue name for temporal worker.
// `downstreamConfig` is a generated configuration for downstream.
// `createService` is user-provided function that creates a struct of handlers.
// `buildDownstreamsClients` is a generated function that creates clients for each downstream.
// `buildServiceHandler` is a generated function that creates temporal service handler.
// temporal service handler itself is a generated struct that wraps downstreams and user handlers.
//
//nolint:funlen
func NewTemporalWorker[
	TemporalServiceHandler, DownstreamConfig, AppConfig, ServiceIntf, Clients any,
	Spec TemporalServiceSpec[TemporalServiceHandler],
](
	ctx context.Context,
	taskQueueName string,
	downstreamConfig DownstreamConfig,
	createService ServiceDefinition[AppConfig, ServiceIntf],
	buildDownstreamsClients func(context.Context, *Hooks) (Clients, error),
	buildServiceHandler func(client.Client, worker.Worker, ServiceIntf, Clients) Spec,
) (StoppableServer, error) {
	// This mirrors what NewServer does
	var externalLogger log.Logger
	ctx, externalLogger = getExternalLogger(ctx)
	if externalLogger == nil {
		ctx = log.PutLogger(ctx, log.NewZeroPkgLogger(zero.New(os.Stdout)).WithStr("bootstrap",
			"logging called before bootstrapping complete, to centralise these logs call "+
				"log.PutLogger before core.NewServer"))
	}

	defaultConfig, appConfig, err := createDefaultConfig(ctx, downstreamConfig, createService)
	if err != nil {
		return nil, err
	}
	ctx = config.PutDefaultConfig(ctx, defaultConfig)

	serviceIntf, hooks, err := createService(ctx, *appConfig)
	if err != nil {
		return nil, err
	}

	if err = validateConfig(ctx, hooks, defaultConfig); err != nil {
		return nil, err
	}

	clientOptions := client.Options{
		HostPort:  defaultConfig.GenCode.Upstream.Temporal.HostPort,
		Namespace: defaultConfig.GenCode.Upstream.Temporal.Namespace,
	}

	if hooks.ExperimentalValidateTemporalClientOptions != nil {
		if err = hooks.ExperimentalValidateTemporalClientOptions(ctx, &clientOptions); err != nil {
			return nil, err
		}
	}

	var temporalClient client.Client
	if hooks.ExperimentalTemporalClientBuilder != nil {
		temporalClient, err = hooks.ExperimentalTemporalClientBuilder(ctx, taskQueueName, &clientOptions)
	} else {
		temporalClient, err = client.Dial(clientOptions)
	}
	if err != nil {
		return nil, err
	}

	downstreamClients, err := buildDownstreamsClients(ctx, hooks)
	if err != nil {
		return nil, err
	}

	workerOptions := worker.Options{
		BackgroundActivityContext: ctx,
	}

	if hooks.ExperimentalValidateTemporalWorkerOptions != nil {
		if err = hooks.ExperimentalValidateTemporalWorkerOptions(ctx, &workerOptions); err != nil {
			return nil, err
		}
	}

	buildWorker := worker.New
	if hooks.ExperimentalTemporalWorkerBuilder != nil {
		buildWorker = hooks.ExperimentalTemporalWorkerBuilder
	}

	return &TemporalServer[TemporalServiceHandler]{
		Spec: buildServiceHandler(
			temporalClient,
			buildWorker(temporalClient, taskQueueName, workerOptions),
			serviceIntf,
			downstreamClients,
		),
	}, nil
}

func createDefaultConfig[DownstreamConfig, AppConfig, Handlers any](
	ctx context.Context,
	downstreamConfig DownstreamConfig,
	createService func(context.Context, AppConfig) (Handlers, *Hooks, error),
) (*config.DefaultConfig, *AppConfig, error) {
	// Load the custom configuration.
	customConfig := NewZeroCustomConfig(reflect.TypeOf(downstreamConfig), GetAppConfigType(createService))
	customConfig, err := LoadCustomConfig(ctx, customConfig)
	if err != nil {
		return nil, nil, err
	}
	if customConfig == nil {
		return nil, nil, fmt.Errorf("configuration is empty")
	}

	customConfigValue := reflect.ValueOf(customConfig).Elem()
	library := customConfigValue.FieldByName("Library").Interface().(config.LibraryConfig)
	admin := customConfigValue.FieldByName("Admin").Interface().(*config.AdminConfig)
	genCodeValue := customConfigValue.FieldByName("GenCode")
	development := customConfigValue.FieldByName("Development").Interface().(*config.DevelopmentConfig)
	upstream := genCodeValue.FieldByName("Upstream").Interface().(config.UpstreamConfig)
	downstreamValue := genCodeValue.FieldByName("Downstream")

	// ensure `downstream` is not nil so that ValidateHooks can use its type
	var downstream any
	if downstreamValue.IsNil() {
		downstream = downstreamConfig
	} else {
		downstream = downstreamValue.Interface()
	}

	appConfigValue := customConfigValue.FieldByName("App").Interface().(AppConfig)

	return &config.DefaultConfig{
		Library:     library,
		Admin:       admin,
		Development: development,
		GenCode: config.GenCodeConfig{
			Upstream:   upstream,
			Downstream: downstream,
		},
	}, &appConfigValue, nil
}

// NewServer returns an auto-generated service.
//
//nolint:funlen
func NewServer(
	ctx context.Context,
	downstreamConfig, createService, serviceInterface interface{},
	newManagers func(ctx context.Context, serviceIntf interface{}, hooks *Hooks) (Manager, *GrpcServerManager, error),
) (StoppableServer, error) {
	// Cache the external logger (i.e. the logger set within the context before bootstrapping).
	var externalLogger log.Logger
	ctx, externalLogger = getExternalLogger(ctx)

	// Put the bootstrap logger in the context if no external logger is provided. The bootstrap
	// logger is designed to provide a fail-safe logger within the context during bootstrapping only.
	// In an ideal setup the bootstrap logger will never be called, however in some edge cases it
	// may be desirable to capture logs before the Hooks.Logger has a chance to be called. In this
	// instance the recommended approach is to call log.PutLogger on the context before calling this
	// method. The outcome of this approach is that the Hooks.Logger method is ignored.
	// The zero pkg logger used below is a deliberate choice because the default logger (the pkg
	// logger) merges fields even when a new set of fields are set against the context therefore
	// this bootstrap message would persist to all log messages in the default case.
	if externalLogger == nil {
		ctx = log.PutLogger(ctx, log.NewZeroPkgLogger(zero.New(os.Stdout)).WithStr("bootstrap",
			"logging called before bootstrapping complete, to centralise these logs call "+
				"log.PutLogger before core.NewServer"))
	}

	// TODO: use the generic config loading function
	// Load the custom configuration.
	MustTypeCheckCreateService(createService, serviceInterface)
	customConfig := NewZeroCustomConfig(reflect.TypeOf(downstreamConfig), GetAppConfigType(createService))
	customConfig, err := LoadCustomConfig(ctx, customConfig)
	if err != nil {
		return nil, err
	}
	if customConfig == nil {
		return nil, fmt.Errorf("configuration is empty")
	}

	customConfigValue := reflect.ValueOf(customConfig).Elem()
	library := customConfigValue.FieldByName("Library").Interface().(config.LibraryConfig)
	admin := customConfigValue.FieldByName("Admin").Interface().(*config.AdminConfig)
	genCodeValue := customConfigValue.FieldByName("GenCode")
	development := customConfigValue.FieldByName("Development").Interface().(*config.DevelopmentConfig)
	appConfig := customConfigValue.FieldByName("App")
	upstream := genCodeValue.FieldByName("Upstream").Interface().(config.UpstreamConfig)
	downstreamValue := genCodeValue.FieldByName("Downstream")

	// ensure `downstream` is not nil so that ValidateHooks can use its type
	var downstream any
	if downstreamValue.IsNil() {
		downstream = downstreamConfig
	} else {
		downstream = downstreamValue.Interface()
	}

	defaultConfig := &config.DefaultConfig{
		Library:     library,
		Admin:       admin,
		Development: development,
		GenCode: config.GenCodeConfig{
			Upstream:   upstream,
			Downstream: downstream,
		},
	}

	// Put the default configuration in the context.
	ctx = config.PutDefaultConfig(ctx, defaultConfig)

	// Create the service by calling the create-service callback.
	createServiceResult := reflect.ValueOf(createService).Call(
		[]reflect.Value{reflect.ValueOf(ctx), appConfig},
	)
	if err := createServiceResult[2].Interface(); err != nil {
		return nil, err.(error)
	}
	serviceIntf := createServiceResult[0].Interface()
	hooksIntf := createServiceResult[1].Interface()

	hooks := hooksIntf.(*Hooks)
	if err = validateConfig(ctx, hooks, defaultConfig); err != nil {
		return nil, err
	}

	// Set the logger against the context if no external logger is provided. The value will be
	// either the value returned from the Hooks (if provided) or the default logger.
	if externalLogger == nil {
		var logger log.Logger
		if hooks != nil && hooks.Logger != nil {
			logger = hooks.Logger()
		}
		if logger == nil {
			logger = log.NewDefaultLogger()
		}
		ctx = log.PutLogger(ctx, logger)
	}

	ctx = withLogLevel(ctx, defaultConfig)

	// Collect prometheus metrics if the admin server is enabled.
	var promRegistry *prometheus.Registry
	if admin != nil {
		promRegistry = prometheus.NewRegistry()
	}

	manager, grpcManager, err := newManagers(ctx, serviceIntf, hooks)
	if err != nil {
		return nil, err
	}

	server := &autogenServer{
		ctx:                ctx,
		name:               "nameless-autogenerated-app", // TODO source the application name from somewhere
		restManager:        manager,
		grpcServerManager:  grpcManager,
		prometheusRegistry: promRegistry,
		multiServer:        nil,
		hooks:              hooks,
	}

	return server, nil
}

// LoadCustomConfig populates the given zero customConfig value with configuration data.
func LoadCustomConfig(ctx context.Context, customConfig interface{}) (interface{}, error) {
	// Figure out where we can read application configuration data from.
	var fs afero.Fs
	var configPath string
	if v := ctx.Value(serveYAMLConfigFileKey); v != nil {
		applicationConfig := v.([]byte)
		fs = afero.NewMemMapFs()
		configPath = "config.yaml"
		err := afero.Afero{Fs: fs}.WriteFile(configPath, applicationConfig, 0777)
		if err != nil {
			return nil, err
		}
	} else {
		fs = afero.NewOsFs()
		if len(os.Args) != 2 {
			return nil, fmt.Errorf("wrong number of arguments (usage: %s (config | -h | --help | -v | --version))", os.Args[0])
		}
		switch os.Args[1] {
		case "--help", "-h":
			fmt.Printf("Usage: %s config\n\n", os.Args[0])
			describeCustomConfig(os.Stdout, customConfig)
			fmt.Print("\n\n")
			return nil, ErrDisplayHelp(2)
		case "--version", "-v":
			fmt.Printf("%s\n", buildMetadata.String())
			return nil, ErrDisplayHelp(2)
		}
		configPath = os.Args[1]
	}

	// Read application configuration data.
	b := config.NewConfigReaderBuilder().WithFs(fs).WithConfigFile(configPath).WithDefaults(config.SetDefaults)

	envPrefixConfigKey := "envPrefix"

	// Enable strict mode to raise an error if there are config keys read from
	// input that have no corresponding place in the customConfig structure
	// that we're going to decode into -- with the exception of the special
	// optional envPrefix key -- that doesn't end up getting decoded into the
	// structure but, if it is present, we do read it below to customise how
	// environment variables are loaded.
	b = b.WithStrictMode(true, envPrefixConfigKey)

	// Use the environment variable prefix from the config file if provided
	env, err := b.Build().GetString(envPrefixConfigKey)
	// Disable the feature if none is provided
	if len(env) > 0 && err == nil {
		b = b.AttachEnvPrefix(env)
	}

	err = b.Build().Unmarshal(customConfig)
	if err != nil {
		return nil, err
	}
	return customConfig, err
}

// NewZeroCustomConfig uses reflection to create a new type derived from DefaultConfig,
// but with new GenCode.Downstream and App fields holding the same types as
// downstreamConfig and appConfig. It returns a pointer to a zero value of that
// new type.
func NewZeroCustomConfig(downstreamConfigType, appConfigType reflect.Type) interface{} {
	defaultConfigType := reflect.TypeOf(config.DefaultConfig{})

	libraryField, has := defaultConfigType.FieldByName("Library")
	if !has {
		panic("config.DefaultType missing Library field")
	}

	adminField, _ := defaultConfigType.FieldByName("Admin")
	if !has {
		panic("config.DefaultType missing Admin field")
	}

	developmentField, has := defaultConfigType.FieldByName("Development")
	if !has {
		panic("config.DefaultType missing Development field")
	}

	genCodeType := reflect.TypeOf(config.GenCodeConfig{})

	upstreamField, has := genCodeType.FieldByName("Upstream")
	if !has {
		panic("config.DefaultType missing Upstream field")
	}

	return reflect.New(reflect.StructOf([]reflect.StructField{
		libraryField,
		adminField,
		{Name: "GenCode", Type: reflect.StructOf([]reflect.StructField{
			upstreamField,
			{Name: "Downstream", Type: downstreamConfigType, Tag: `mapstructure:"downstream"`},
		}), Tag: `mapstructure:"genCode"`},
		developmentField,
		{Name: "App", Type: appConfigType, Tag: `mapstructure:"app"`},
	})).Interface()
}

// MustTypeCheckCreateService checks that the given createService has an acceptable type, and panics otherwise.
func MustTypeCheckCreateService(createService, serviceInterface interface{}) {
	cs := reflect.TypeOf(createService)
	if cs.NumIn() != 2 {
		panic("createService: wrong number of in params")
	}
	if cs.NumOut() != 3 {
		panic("createService: wrong number of out params")
	}

	var ctx context.Context
	if reflect.TypeOf(&ctx).Elem() != cs.In(0) {
		panic(fmt.Errorf("createService: first in param must be of type context.Context, not %v", cs.In(0)))
	}

	serviceInterfaceType := reflect.TypeOf(serviceInterface)
	if serviceInterfaceType != cs.Out(0) {
		panic(fmt.Errorf("createService: second out param must be of type %v, not %v", serviceInterfaceType, cs.Out(0)))
	}

	var hooks Hooks
	if reflect.TypeOf(&hooks) != cs.Out(1) {
		panic(fmt.Errorf("createService: second out param must be of type *Hooks, not %v", cs.Out(1)))
	}

	var err error
	if reflect.TypeOf(&err).Elem() != cs.Out(2) {
		panic(fmt.Errorf("createService: third out param must be of type error, not %v", cs.Out(1)))
	}
}

// GetAppConfigType extracts the app's config type from createService.
// Precondition: MustTypeCheckCreateService(createService, serviceInterface) succeeded.
func GetAppConfigType(createService interface{}) reflect.Type {
	cs := reflect.TypeOf(createService)
	return cs.In(1)
}

func yamlEgComment(example, format string, args ...interface{}) string {
	return fmt.Sprintf("\033[1;31m%s \033[0;32m# "+format+"\033[0m", append([]interface{}{example}, args...)...)
}

func describeCustomConfig(w io.Writer, customConfig interface{}) {
	commonTypes := map[reflect.Type]string{
		reflect.TypeOf(config.CommonServerConfig{}):   "",
		reflect.TypeOf(config.CommonDownstreamData{}): "",
		reflect.TypeOf(config.TLSConfig{}):            "",
		reflect.TypeOf(config.SensitiveString{}):      yamlEgComment(`"*****"`, "sensitive string"),
	}

	fmt.Fprint(w, "\033[1mConfiguration file YAML schema\033[0m")

	commonTypeNames := make([]string, 0, len(commonTypes))
	commonTypesByName := make(map[string]reflect.Type, len(commonTypes))
	for ct := range commonTypes {
		name := fmt.Sprintf("%s.%s", ct.PkgPath(), ct.Name())
		commonTypeNames = append(commonTypeNames, name)
		commonTypesByName[name] = ct
	}
	sort.Strings(commonTypeNames)

	for _, name := range commonTypeNames {
		ct := commonTypesByName[name]
		if commonTypes[ct] == "" {
			delete(commonTypes, ct)
			fmt.Fprintf(w, "\n\n\033[1;32m%q.%s:\033[0m", ct.PkgPath(), ct.Name())
			describeYAMLForType(w, ct, commonTypes, 4)
			commonTypes[ct] = ""
		}
	}

	fmt.Fprintf(w, "\n\n\033[1mApplication Configuration\033[0m")
	describeYAMLForType(w, reflect.TypeOf(customConfig), commonTypes, 0)
}

//nolint:funlen
func describeYAMLForType(w io.Writer, t reflect.Type, commonTypes map[reflect.Type]string, indent int) {
	outf := func(format string, args ...interface{}) {
		parts := strings.SplitAfterN(format, "\n", 2)
		fmt.Fprintf(w, strings.Join(parts, strings.Repeat(" ", indent)), args...)
	}
	if alias, has := commonTypes[t]; has {
		if alias == "" {
			outf(" " + yamlEgComment(`{}`, "%q.%s", t.PkgPath(), t.Name()))
		} else {
			outf(" %s", alias)
		}
		return
	}
	switch t.Kind() {
	case reflect.Bool:
		outf(" \033[1mfalse\033[0m")
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		outf(" \033[1m0\033[0m")
	case reflect.Float32, reflect.Float64:
		outf(" \033[1m0.0\033[0m")
	case reflect.Array, reflect.Slice:
		outf("\n-")
		describeYAMLForType(w, t.Elem(), commonTypes, indent+4)
	case reflect.Interface:
		outf(" " + yamlEgComment("{}", "any value"))
	case reflect.Map:
		outf("\n key: ")
		describeYAMLForType(w, t.Elem(), commonTypes, indent+4)
	case reflect.Ptr:
		describeYAMLForType(w, t.Elem(), commonTypes, indent)
	case reflect.String:
		outf(" \033[1m\"\"\033[0m")
	case reflect.Struct:
		n := t.NumField()
		for i := 0; i < n; i++ {
			f := t.Field(i)
			var name string
			yamlTag := f.Tag.Get("yaml")
			yamlParts := strings.Split(yamlTag, ",")
			mapTag := f.Tag.Get("mapstructure")
			mapParts := strings.Split(mapTag, ",")
			switch {
			case len(mapTag) > 0:
				name = mapParts[0]
			case len(yamlTag) > 0:
				name = yamlParts[0]
			case f.Type.Kind() == reflect.Func:
				name = ""
			default:
				name = f.Name
			}
			if len(name) > 0 {
				outf("\n%s:", name)
			}
			describeYAMLForType(w, f.Type, commonTypes, indent+4)
		}
	case reflect.Func:
		break
	default:
		panic(fmt.Errorf("describeYAMLForType: Unhandled type: %v", t))
	}
}

type autogenServer struct {
	ctx                context.Context
	name               string
	restManager        Manager
	grpcServerManager  *GrpcServerManager
	prometheusRegistry *prometheus.Registry
	multiServer        StoppableServer
	hooks              *Hooks
	m                  sync.Mutex // protect access to multiServer
}

//nolint:funlen,gocognit
func (s *autogenServer) Start() error {
	// precondition: ctx must have been threaded through InitialiseLogging and hence contain a logger
	ctx := s.ctx

	// prepare the middleware
	var contextTimeout time.Duration
	if s.restManager != nil && s.restManager.PublicServerConfig() != nil {
		contextTimeout = s.restManager.PublicServerConfig().ContextTimeout
	}
	if contextTimeout == 0 {
		contextTimeout = defaultContextTimeout
	}
	mWare := prepareMiddleware(s.name, s.prometheusRegistry, contextTimeout)

	// load health server
	var healthServer *health.Server
	var err error
	if s.restManager != nil && s.restManager.LibraryConfig() != nil && s.restManager.LibraryConfig().Health {
		healthServer, err = health.NewServer()
		if err != nil {
			return err
		}
		s.grpcServerManager.EnabledGrpcHandlers = append(s.grpcServerManager.EnabledGrpcHandlers, healthServer)
	}

	var restIsRunning, grpcIsRunning bool

	servers := make([]StoppableServer, 0)

	// Make the listener function for the REST Admin server
	if s.restManager != nil && s.restManager.AdminServerConfig() != nil {
		log.Info(ctx, "found AdminServerConfig for REST")
		var healthHTTPServer *pkgHealth.HTTPServer
		if healthServer != nil {
			healthHTTPServer = healthServer.HTTP
		}
		serverAdmin, err := configureAdminServerListener(ctx, s.restManager, s.prometheusRegistry, healthHTTPServer, mWare.admin)
		if err != nil {
			return err
		}
		servers = append(servers, serverAdmin)
	} else {
		log.Info(ctx, "no AdminServerConfig for REST was found")
	}

	// Make the listener function for the REST Public server
	if s.restManager != nil && s.restManager.PublicServerConfig() != nil {
		log.Info(ctx, "found PublicServerConfig for REST")
		serverPublic, err := configurePublicServerListener(ctx, s.restManager, mWare.public, s.hooks)
		if err != nil {
			return err
		}
		servers = append(servers, serverPublic)
		restIsRunning = true
	} else {
		log.Info(ctx, "no PublicServerConfig for REST was found")
	}

	// Make the listener function for the gRPC Public server.
	if s.grpcServerManager != nil && s.grpcServerManager.GrpcPublicServerConfig != nil && len(s.grpcServerManager.EnabledGrpcHandlers) > 0 {
		log.Info(ctx, "found GrpcPublicServerConfig for gRPC")
		serverPublicGrpc := configurePublicGrpcServerListener(ctx, *s.grpcServerManager, s.hooks)
		servers = append(servers, serverPublicGrpc)
		grpcIsRunning = true
	} else {
		log.Info(ctx, "no GrpcPublicServerConfig for gRPC was found")
	}

	// Refuse to start and panic if neither of the public servers are enabled.
	if !restIsRunning && !grpcIsRunning {
		err := errors.New("REST and gRPC servers cannot both be nil")
		log.Error(ctx, err, "error starting server")
		panic(err)
	}

	s.newMultiStoppableServer(ctx, servers)

	if healthServer != nil {
		healthServer.SetReady(true)
	}

	return s.multiServer.Start()
}

func (s *autogenServer) newMultiStoppableServer(ctx context.Context, servers []StoppableServer) {
	s.m.Lock()
	defer s.m.Unlock()
	s.multiServer = &multiStoppableServer{ctx, servers}
}

// FIXME replace MultiError with some existing type that does this job better.
type MultiError struct {
	Msg    string
	Errors []error
}

func (e MultiError) Error() string {
	msgs := make([]string, len(e.Errors))
	for i, e := range e.Errors {
		msgs[i] = e.Error()
	}
	return fmt.Sprintf("%s; sub-error(s): %s", e.Msg, strings.Join(msgs, "; "))
}

func (s *autogenServer) Stop() error {
	s.m.Lock()
	defer s.m.Unlock()

	if s.multiServer == nil {
		return nil
	}

	return s.multiServer.Stop()
}

func (s *autogenServer) GracefulStop() error {
	s.m.Lock()
	defer s.m.Unlock()

	if s.multiServer == nil {
		return nil
	}

	return s.multiServer.GracefulStop()
}

func (s *autogenServer) GetName() string {
	return s.name
}

type multiStoppableServer struct {
	ctx     context.Context
	servers []StoppableServer
}

func NewMultiStoppableServer(ctx context.Context, servers []StoppableServer) StoppableServer {
	return &multiStoppableServer{ctx, servers}
}

func (s *multiStoppableServer) Start() error {
	// precondition: ctx must have been threaded through InitialiseLogging and hence contain a logger
	ctx := s.ctx

	// Start all configured servers and block until the first one terminates.
	errChan := make(chan error, 1)
	for i := range s.servers {
		i := i                 // force capture
		server := s.servers[i] // force capture
		go func() {
			log.Infof(ctx, "starting sub-server %d of %d (%s)", i+1, len(s.servers), server.GetName())
			defer func() {
				if r := recover(); r != nil {
					errChan <- fmt.Errorf("server %d (%s) panicked: %v", i+1, server.GetName(), r)
				}
			}()

			err := server.Start()
			if err != nil {
				err = fmt.Errorf("server %d (%s) returned an error: %v", i+1, server.GetName(), err)
			}
			errChan <- err
		}()
	}

	return <-errChan
}

func (s *multiStoppableServer) Stop() error {
	errQueue := make(chan error, len(s.servers))

	var wg sync.WaitGroup
	for i := range s.servers {
		i := i                 // force capture
		server := s.servers[i] // force capture
		wg.Add(1)
		go func() {
			defer wg.Done()
			log.Infof(s.ctx, "stopping sub-server %d of %d (%s)...", i+1, len(s.servers), server.GetName())
			err := server.Stop()
			log.Infof(s.ctx, "stopped sub-server %d of %d (%s)", i+1, len(s.servers), server.GetName())
			if err != nil {
				errQueue <- err
			}
		}()
	}
	wg.Wait()
	close(errQueue)
	errs := make([]error, 0)
	for err := range errQueue {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return MultiError{Msg: "error during stop", Errors: errs}
	}
	return nil
}

func (s *multiStoppableServer) GracefulStop() error {
	errQueue := make(chan error, len(s.servers))

	var wg sync.WaitGroup
	for i := range s.servers {
		i := i                 // force capture
		server := s.servers[i] // force capture
		wg.Add(1)
		go func() {
			defer wg.Done()
			log.Infof(s.ctx, "graceful-stopping sub-server %d of %d (%s)...", i+1, len(s.servers), server.GetName())
			err := server.GracefulStop()
			log.Infof(s.ctx, "graceful-stopped sub-server %d of %d (%s)", i+1, len(s.servers), server.GetName())
			if err != nil {
				errQueue <- err
			}
		}()
	}
	wg.Wait()
	close(errQueue)
	errs := make([]error, 0)
	for err := range errQueue {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return MultiError{Msg: "error during graceful stop", Errors: errs}
	}
	return nil
}

func (s multiStoppableServer) GetName() string {
	return "multiStoppableServer"
}

// getExternalLogger returns the log.Logger instance that has been set within the context before
// Sysl-go is initialised. This method is designed to support a variety of use-cases:
//
// 1. The recommended approach to customise the logging implementation is to provide a logger
// instance through the Hooks.Logger method. The logger is set against the context and made
// available throughout the application through various methods (e.g. log.Info). In this
// configuration (i.e. no external logger has been set), this method returns nil.
//
// 2. Due to the way Sysl-go initialises its services, in some edge cases it is desirable to capture
// logs before the Hooks.Logger has a chance to be called. In this instance the recommendation is to
// call log.PutLogger on the context before Sysl-go bootstrapping. The outcome of this approach is
// that the Hooks.Logger method is ignored. In this configuration (i.e. the external logger directly
// set), this method returns the logger.
//
// 3. In order to support legacy Sysl-go applications that directly set a Logrus logger against the
// context before bootstrapping, this method will wrap and return an appropriate log.Logger instance.
//
// 4. In order to support legacy Sysl-go applications that directly set a pkg logger against the
// context before bootstrapping, this method will wrap and return an appropriate log.Logger instance.
//
// 5. For other configurations this method will return nil.
func getExternalLogger(ctx context.Context) (context.Context, log.Logger) {
	logger := log.GetLogger(ctx)
	if logger != nil {
		return ctx, logger
	}
	logrus := log.GetLogrusLoggerFromContext(ctx)
	if logrus != nil {
		lgr := log.NewLogrusLogger(logrus)
		lgr.Debug("legacy logrus logger configuration detected, use Hooks.Logger instead")
		return log.PutLogger(ctx, lgr), lgr
	}
	fields := pkg.FieldsFrom(ctx)
	empty := pkg.Fields{}
	if fields != empty {
		lgr := log.NewPkgLogger(fields)
		lgr.Debug("legacy pkg logger configuration detected, use Hooks.Logger instead")
		return log.PutLogger(ctx, lgr), lgr
	}
	return ctx, nil
}

// func createDefaultConfig()
