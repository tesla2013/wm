package main

import (
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/docopt/docopt-go"
)

type DatePath struct {
	year  int
	month int
	day   int
}

func parseDateString(in_date string) (*DatePath, error) {
	in_date = strings.ToLower(in_date)
	in_date = strings.TrimSpace(in_date)

	if in_date == "today" || len(in_date) == 0 {
		return &DatePath{
			year:  time.Now().Year(),
			month: int(time.Now().Month()),
			day:   time.Now().Day()}, nil
	} else if in_date == "yesterday" {
		yest := time.Now().Add(-1 * time.Hour * 24)
		return &DatePath{
			year:  yest.Year(),
			month: int(yest.Month()),
			day:   yest.Day(),
		}, nil
	} else if in_date == "tomorrow" {
		tom := time.Now().Add(time.Hour * 24)
		return &DatePath{
			year:  tom.Year(),
			month: int(tom.Month()),
			day:   tom.Day(),
		}, nil
	}

	date_formats := []string{
		"1/2/2006",
		"1-2-2006",
		"Jan 2 2006",
		"Jan 2, 2006",
		"2 Jan 2006",
		"2 Jan, 2006",
		"2/1/2006",
		"2-1-2006",
		"2-Jan-2006",
		"January 2 2006",
		"January 2, 2006",
		"2 January 2006",
		"2 January, 2006",
	}

	for _, df := range date_formats {
		pd, err := time.Parse(df, in_date)
		if err != nil {
			continue
		}
		return &DatePath{
			year:  pd.Year(),
			month: int(pd.Month()),
			day:   pd.Day(),
		}, nil
	}

	return nil, fmt.Errorf("unable to parse '%s'", in_date)

}

func (ds *DatePath) String() string {
	return fmt.Sprintf("/%d/%d/%d.txt", ds.year, ds.month, ds.day)
}

type Configuration struct {
	Root   string
	Editor string
}

func main() {
	usage := `WM.  A working-memory log system.

WM will open the log file for the day provided.  If none is provided, the
current date is assumed.  If the file for the provided date already exists, it
is opened; if not, it is created first.  The program that is used to open the
log is defined in the configuration file.  By invoking the 'config' command,
the configuration file is opened for editing.

Configuration is done using a TOML file with the following recoginzed keys.
	root	A string representing the complete path to the root folder for
		working memory logs.  Default is '~/.wm/logs'
	editor	A string for the file path of the program to edit working
		memory logs.

The configuration file is stored next to the executable file itself by default
but can be changed by providing a WMCFG environment variable.

Provide "search" space separated terms to search the working memory database for.
A table of results that includes all hits will be provided ordered by date.

Usage:
  wm [<date>]
  wm -c | --config
  wm -h | --help
  wm --version
  wm search <term>...

Options:
  -c --config   Configure WM
  -h --help     Display this screen
  --version     Display the current version`

	opts, err := docopt.ParseArgs(usage, nil, "0.2.0")
	if err != nil {
		log.Fatalln("could not parse arguments:", err)
	}
	d, _ := opts.String("<date>")
	c, _ := opts.Bool("--config")

	cfgFile := os.Getenv("WMCFG")
	if len(cfgFile) == 0 {
		cfgFile = "wm.toml"
	}
	if _, err := os.Stat(cfgFile); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			f, err := os.Create(cfgFile)
			if err != nil {
				log.Fatalln("config file not found at '", cfgFile, "' and failed to create.")
			}
			defer f.Close()
			_, err = f.WriteString(`root = "~/.wm/logs"
editor = "notepad"`)
			if err != nil {
				log.Fatalln("config file not found at '", cfgFile, "'. Created, but failed to write defaults.")
			}
		} else {
			log.Fatalln("failed to verify configuration file exists:", err)
		}
	}

	cfgData, err := os.ReadFile(cfgFile)
	if err != nil {
		log.Fatalln("error reading config file:", err)
	}
	var cfg Configuration
	_, err = toml.Decode(string(cfgData), &cfg)
	if err != nil {
		log.Fatalln("error decoding configuration file:", err)
	}

	if c {
		cmd := exec.Command(cfg.Editor, cfgFile)
		err = cmd.Start()
		if err != nil {
			log.Fatalln("failed to open configuration file using", cfg.Editor, ":", err)
		}
		err = cmd.Wait()
		if err != nil {
			log.Fatalln("configuration failed to update:", err)
		}
		os.Exit(0)
	}

	pd, err := parseDateString(d)
	if err != nil {
		log.Fatalln("error parsing date:", err)
	}
	wmPath := cfg.Root + pd.String()
	if strings.Contains(wmPath, "~/") {
		hd, err := os.UserHomeDir()
		if err != nil {
			log.Fatalln("failed to convert '~' to the users home directory:", err)
		}
		wmPath = strings.Replace(wmPath, "~/", hd+"/", 1)
		wmPath = strings.ReplaceAll(wmPath, "/", "\\")
	}
	wmDir := filepath.Dir(wmPath)
	err = os.MkdirAll(wmDir, fs.ModeDir)
	if err != nil {
		log.Fatalln("failed to create directory for working memory file:", err)
	}

	if _, err := os.Stat(wmPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			f, err := os.Create(wmPath)
			if err != nil {
				log.Fatalln("working memory file not found at '", wmPath, "' and failed to create:", err)
			}
			defer f.Close()
			_, err = f.WriteString(fmt.Sprintf(`Working Memory File
%d/%d/%d
-------------------

`, pd.month, pd.day, pd.year))
			if err != nil {
				log.Fatalln("working memory file not found at '", wmPath, "'. Created, but failed to write defaults.")
			}
		} else {
			log.Fatalln("failed to verify working memory file exists:", err)
		}
	}

	cmd := exec.Command(cfg.Editor, wmPath)
	err = cmd.Start()
	if err != nil {
		log.Fatalln("failed to open working memory file using", cfg.Editor, ":", err)
	}
	os.Exit(0)

}
