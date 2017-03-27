package main

import (
	"fmt"
	"github.com/Comcast/webpa-common/concurrent"
	"github.com/Comcast/webpa-common/handler"
	"github.com/Comcast/webpa-common/secure"
	"github.com/Comcast/webpa-common/server"
	"github.com/gorilla/mux"
	"github.com/justinas/alice"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"os"
	"os/signal"
	"time"
)

const (
	applicationName = "caduceus"
)

// caduceus is the driver function for Caduceus.  It performs everything main() would do,
// except for obtaining the command-line arguments (which are passed to it).
func caduceus(arguments []string) int {
	var (
		f = pflag.NewFlagSet(applicationName, pflag.ContinueOnError)
		v = viper.New()

		logger, webPA, err = server.Initialize(applicationName, arguments, f, v)
	)

	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to initialize Viper environment: %s\n", err)
		return 1
	}

	logger.Info("Using configuration file: %s", v.ConfigFileUsed())

	caduceusConfig := new(CaduceusConfig)
	err = v.Unmarshal(caduceusConfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to unmarshal configuration data into struct: %s\n", err)
		return 1
	}

	workerPool := WorkerPoolFactory{
		NumWorkers: caduceusConfig.NumWorkerThreads,
		QueueSize:  caduceusConfig.JobQueueSize,
	}.New()

	caduceusProfilerFactory := ServerProfilerFactory{
		Frequency: caduceusConfig.ProfilerFrequency,
		Duration:  caduceusConfig.ProfilerDuration,
		QueueSize: caduceusConfig.ProfilerQueueSize,
	}

	// declare a new sender wrapper and pass it a profiler factory so that it can create
	// unique profilers on a per outboundSender basis
	caduceusSenderWrapper, err := SenderWrapperFactory{
		NumWorkersPerSender: caduceusConfig.SenderNumWorkersPerSender,
		QueueSizePerSender:  caduceusConfig.SenderQueueSizePerSender,
		CutOffPeriod:        time.Duration(caduceusConfig.SenderCutOffPeriod) * time.Second,
		Linger:              time.Duration(caduceusConfig.SenderLinger) * time.Second,
		ProfilerFactory:     caduceusProfilerFactory,
		Logger:              logger,
		// TODO: ask Wes how I should be setting this
		// Client: ,
	}.New()

	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to initialize new caduceus sender wrapper: %s\n", err)
		return 1
	}

	// here we create a profiler specifically for our main server handler
	caduceusHandlerProfiler, err := caduceusProfilerFactory.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to profiler for main caduceus handler: %s\n", err)
		return 1
	}

	serverWrapper := &ServerHandler{
		Logger: logger,
		caduceusHandler: &CaduceusHandler{
			handlerProfiler: caduceusHandlerProfiler,
			senderWrapper:   caduceusSenderWrapper,
			Logger:          logger,
		},
		doJob: workerPool.Send,
	}

	profileWrapper := &ProfileHandler{
		profilerData: caduceusHandlerProfiler,
		Logger:       logger,
	}

	validator := secure.Validators{
		secure.ExactMatchValidator(caduceusConfig.AuthHeader),
	}

	authHandler := handler.AuthorizationHandler{
		HeaderName:          "Authorization",
		ForbiddenStatusCode: 403,
		Validator:           validator,
		Logger:              logger,
	}

	caduceusHandler := alice.New(authHandler.Decorate)

	mux := mux.NewRouter()
	mux.Handle("/api/v1/run", caduceusHandler.Then(serverWrapper))
	mux.Handle("/api/v1/profile", caduceusHandler.Then(profileWrapper))

	caduceusHealth := &CaduceusHealth{}
	var runnable concurrent.Runnable

	caduceusHealth.Monitor, runnable = webPA.Prepare(logger, mux)
	serverWrapper.caduceusHealth = caduceusHealth

	waitGroup, shutdown, err := concurrent.Execute(runnable)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to start device manager: %s\n", err)
		return 1
	}

	logger.Info("Caduceus is up and running!")

	var (
		signals = make(chan os.Signal, 1)
	)

	signal.Notify(signals)
	<-signals
	close(shutdown)
	waitGroup.Wait()

	// shutdown the sender wrapper gently so that all queued messages get serviced
	caduceusSenderWrapper.Shutdown(true)

	return 0
}

func main() {
	os.Exit(caduceus(os.Args))
}
