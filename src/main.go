// The Main function of HTTPMS. It should set everything up, create a library and
// create a webserver.
//
// At the moment it is in package src because I import it from the project's root
// folder.
package src

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"path/filepath"

	"github.com/ironsmile/httpms/src/config"
	"github.com/ironsmile/httpms/src/daemon"
	"github.com/ironsmile/httpms/src/helpers"
	"github.com/ironsmile/httpms/src/library"
	"github.com/ironsmile/httpms/src/webserver"
)

var (
	PidFile string
	Debug   bool
)

func init() {
	pidUsage := "Pidfile. Default is [user_path]/pidfile.pid"
	pidDefault := "pidfile.pid"
	flag.StringVar(&PidFile, "p", pidDefault, pidUsage)

	flag.BoolVar(&Debug, "D", false, "Debug mode. Will log everything to the stdout.")
}

// This function is the only thing run in the project's root main.go file.
// For all intent and purposes this is the main function.
func Main() {
	flag.Parse()

	projRoot, err := helpers.ProjectRoot()
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}

	err = ParseConfigAndStartWebserver(projRoot)

	if err != nil {
		log.Println(err)
		os.Exit(1)
	}
}

// Creates a pidfile and starts a signal receiver goroutine
func SetupPidFileAndSignals(pidFile string) {
	helpers.SetUpPidFile(pidFile)

	signalChannel := make(chan os.Signal, 2)
	for _, sig := range daemon.StopSignals {
		signal.Notify(signalChannel, sig)
	}
	go func() {
		for _ = range signalChannel {
			log.Println("Stop signal received. Removing pidfile and stopping.")
			helpers.RemovePidFile(pidFile)
			os.Exit(0)
		}
	}()
}

// Returns a new Library object using the application config.
// For the moment this is a LocalLibrary which will place its sqlite db file
// in the UserPath directory
func getLibrary(userPath string, cfg config.Config) (library.Library, error) {
	dbPath := helpers.AbsolutePath(cfg.SqliteDatabase, userPath)
	lib, err := library.NewLocalLibrary(dbPath)

	if err != nil {
		return nil, err
	}

	err = lib.Initialize()

	if err != nil {
		return nil, err
	}

	for _, path := range cfg.Libraries {
		lib.AddLibraryPath(path)
	}

	return lib, nil
}

// Parses the config, sets the logfile, setups the pidfile, and makes an
// signal handler goroutine
func ParseConfigAndStartWebserver(projRoot string) error {

	var cfg config.Config
	err := cfg.FindAndParse()

	if err != nil {
		return err
	}

	userPath := filepath.Dir(cfg.UserConfigPath())

	if !Debug {
		err = helpers.SetLogsFile(helpers.AbsolutePath(cfg.LogFile, userPath))
		if err != nil {
			return err
		}
	}

	pidFile := helpers.AbsolutePath(PidFile, userPath)
	SetupPidFileAndSignals(pidFile)
	defer helpers.RemovePidFile(pidFile)

	lib, err := getLibrary(userPath, cfg)
	if err != nil {
		return err
	}
	lib.Scan()

	cfg.HTTPRoot = helpers.AbsolutePath(cfg.HTTPRoot, projRoot)

	srv := webserver.NewServer(cfg, lib)
	srv.Serve()
	srv.Wait()
	return nil
}
