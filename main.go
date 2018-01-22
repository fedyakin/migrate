// Package main is the CLI.
// You can use the CLI via Terminal.
// import "github.com/mattes/migrate/migrate" for usage within Go.
package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/fatih/color"
	"github.com/healthimation/go-aws-config/src/awsconfig"
	"github.com/turbine/migrate/driver"
	"github.com/turbine/migrate/file"
	"github.com/turbine/migrate/migrate"
	"github.com/turbine/migrate/migrate/direction"
	pipep "github.com/turbine/migrate/pipe"
)

var url = flag.String("url", "", "")
var migrationsPath = flag.String("path", "", "")
var version = flag.Bool("version", false, "Show migrate version")
var transactionType = flag.String("txn", "PerFile", "")
var environment = flag.String("env", "", "The environment you're running in")
var service = flag.String("service", "", "The service's name (combined with env to form AWS parameter store key \"/env/service/urlkey\")")
var dbURLKey = flag.String("urlkey", "DB_URL", "")

func main() {
	flag.Parse()
	command := flag.Arg(0)

	if *version {
		fmt.Println(Version)
		os.Exit(0)
	}

	if *migrationsPath == "" {
		*migrationsPath, _ = os.Getwd()
	}

	txnType, err := driver.GetTxnType(*transactionType)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	// check AWS parameter for db URL
	if len(*url) == 0 && len(*environment) > 0 && len(*service) > 0 {
		conf := awsconfig.NewAWSLoader(*environment, *service)
		err := conf.Initialize()
		if err != nil {
			fmt.Printf("Could not pull AWS parameter store configuration: %s\n", err.Error())
			os.Exit(1)
		}
		dbURLby, err := conf.Get(*dbURLKey)
		if err != nil || len(dbURLby) == 0 {
			fmt.Printf("AWS parameter store key /%s/%s/%s was missing or not parsable\n", *environment, *service, *dbURLKey)
			os.Exit(1)
		}
		dbURL := string(dbURLby)
		url = &dbURL
	}

	switch command {
	case "create":
		verifyMigrationsPath(*migrationsPath)
		name := flag.Arg(1)
		if name == "" {
			fmt.Println("Please specify name.")
			os.Exit(1)
		}

		migrationFile, err := migrate.Create(*url, *migrationsPath, name, txnType)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		fmt.Printf("Version %v migration files created in %v:\n", migrationFile.Version, *migrationsPath)
		fmt.Println(migrationFile.UpFile.FileName)
		fmt.Println(migrationFile.DownFile.FileName)

	case "migrate":
		verifyMigrationsPath(*migrationsPath)
		relativeN := flag.Arg(1)
		relativeNInt, err := strconv.Atoi(relativeN)
		if err != nil {
			fmt.Println("Unable to parse param <n>.")
			os.Exit(1)
		}
		timerStart = time.Now()
		pipe := pipep.New()
		go migrate.Migrate(pipe, *url, *migrationsPath, relativeNInt, txnType)
		ok := writePipe(pipe)
		printTimer()
		if !ok {
			os.Exit(1)
		}

	case "goto":
		verifyMigrationsPath(*migrationsPath)
		toVersion := flag.Arg(1)
		toVersionInt, err := strconv.Atoi(toVersion)
		if err != nil || toVersionInt < 0 {
			fmt.Println("Unable to parse param <v>.")
			os.Exit(1)
		}

		currentVersion, err := migrate.Version(*url, *migrationsPath, txnType)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		relativeNInt := toVersionInt - int(currentVersion)

		timerStart = time.Now()
		pipe := pipep.New()
		go migrate.Migrate(pipe, *url, *migrationsPath, relativeNInt, txnType)
		ok := writePipe(pipe)
		printTimer()
		if !ok {
			os.Exit(1)
		}

	case "up":
		verifyMigrationsPath(*migrationsPath)
		timerStart = time.Now()
		pipe := pipep.New()
		go migrate.Up(pipe, *url, *migrationsPath, txnType)
		ok := writePipe(pipe)
		printTimer()
		if !ok {
			os.Exit(1)
		}

	case "down":
		verifyMigrationsPath(*migrationsPath)
		timerStart = time.Now()
		pipe := pipep.New()
		go migrate.Down(pipe, *url, *migrationsPath, txnType)
		ok := writePipe(pipe)
		printTimer()
		if !ok {
			os.Exit(1)
		}

	case "redo":
		verifyMigrationsPath(*migrationsPath)
		timerStart = time.Now()
		pipe := pipep.New()
		go migrate.Redo(pipe, *url, *migrationsPath, txnType)
		ok := writePipe(pipe)
		printTimer()
		if !ok {
			os.Exit(1)
		}

	case "reset":
		verifyMigrationsPath(*migrationsPath)
		timerStart = time.Now()
		pipe := pipep.New()
		go migrate.Reset(pipe, *url, *migrationsPath, txnType)
		ok := writePipe(pipe)
		printTimer()
		if !ok {
			os.Exit(1)
		}

	case "version":
		verifyMigrationsPath(*migrationsPath)
		version, err := migrate.Version(*url, *migrationsPath, txnType)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		fmt.Println(version)

	default:
		fallthrough
	case "help":
		helpCmd()
	}
}

func writePipe(pipe chan interface{}) (ok bool) {
	okFlag := true
	if pipe != nil {
		for {
			select {
			case item, more := <-pipe:
				if !more {
					return okFlag
				} else {
					switch item.(type) {

					case string:
						fmt.Println(item.(string))

					case error:
						c := color.New(color.FgRed)
						c.Println(item.(error).Error(), "\n")
						okFlag = false

					case file.File:
						f := item.(file.File)
						c := color.New(color.FgBlue)
						if f.Direction == direction.Up {
							c.Print(">")
						} else if f.Direction == direction.Down {
							c.Print("<")
						}
						fmt.Printf(" %s\n", f.FileName)

					default:
						text := fmt.Sprint(item)
						fmt.Println(text)
					}
				}
			}
		}
	}
	return okFlag
}

func verifyMigrationsPath(path string) {
	if path == "" {
		fmt.Println("Please specify path")
		os.Exit(1)
	}
}

var timerStart time.Time

func printTimer() {
	diff := time.Now().Sub(timerStart).Seconds()
	if diff > 60 {
		fmt.Printf("\n%.4f minutes\n", diff/60)
	} else {
		fmt.Printf("\n%.4f seconds\n", diff)
	}
}

func helpCmd() {
	os.Stderr.WriteString(
		`usage: migrate [-path=<path>] [-url=<url>] [-env=<environment> -service=<serviceName> [-urlkey=<urlkey>]] <command> [<args>]

Commands:
   create <name>  Create a new migration
   up             Apply all -up- migrations
   down           Apply all -down- migrations
   reset          Down followed by Up
   redo           Roll back most recent migration, then apply it again
   version        Show current migration version
   migrate <n>    Apply migrations -n|+n
   goto <v>       Migrate to version v
   help           Show this help

'-path' defaults to current working directory.
'-url' or '-env' and '-service' are required.  
If you provide '-env' and '-service' the app will look up the '-urlkey' in AWS parameter store as '/env/service/urlkey'
You will need to provide a standard AWS credential provider to use the '-env' and '-service' parameters.
'-urlkey' defaults to DB_URL
`)
}
