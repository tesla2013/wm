package main

import (
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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

type Parameters struct {
	Config bool
	Search bool
	Term   []string
	Date   string
}

func parseDateString(inDate string) (*DatePath, error) {
	inDate = strings.ToLower(inDate)
	inDate = strings.TrimSpace(inDate)

	if inDate == "today" || len(inDate) == 0 {
		return &DatePath{
			year:  time.Now().Year(),
			month: int(time.Now().Month()),
			day:   time.Now().Day()}, nil
	} else if inDate == "yesterday" {
		yest := time.Now().Add(-1 * time.Hour * 24)
		return &DatePath{
			year:  yest.Year(),
			month: int(yest.Month()),
			day:   yest.Day(),
		}, nil
	} else if inDate == "tomorrow" {
		tom := time.Now().Add(time.Hour * 24)
		return &DatePath{
			year:  tom.Year(),
			month: int(tom.Month()),
			day:   tom.Day(),
		}, nil
	}

	dateFormats := []string{
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

	for _, df := range dateFormats {
		pd, err := time.Parse(df, inDate)
		if err != nil {
			continue
		}
		return &DatePath{
			year:  pd.Year(),
			month: int(pd.Month()),
			day:   pd.Day(),
		}, nil
	}

	return nil, fmt.Errorf("unable to parse '%s'", inDate)

}

func (ds *DatePath) String() string {
	return fmt.Sprintf("/%d/%d/%d.txt", ds.year, ds.month, ds.day)
}

type Configuration struct {
	Root        string
	Editor      string
	ContextSize int
}

func GetConfig(cfgFile string) Configuration {
	if _, err := os.Stat(cfgFile); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			f, err := os.Create(cfgFile)
			if err != nil {
				log.Fatalln("config file not found at '", cfgFile, "' and failed to create.")
			}
			_, err = f.WriteString(`root = "~/.wm/logs"
editor = "notepad"
context_size = 200`)
			if err != nil {
				log.Fatalln("config file not found at '", cfgFile, "'. Created, but failed to write defaults.")
			}
			err = f.Close()
			if err != nil {
				log.Fatalln("failed to close file with error ", err)
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
	return cfg
}

func main() {
	usage := `WM.  A working-memory log system.

WM will open the log file for the day provided.  If none is provided, the
current date is assumed.  If the file for the provided date already exists, it
is opened; if not, it is created first.  The program that is used to open the
log is defined in the configuration file.  By invoking the 'config' command,
the configuration file is opened for editing.

Configuration is done using a TOML file with the following recognized keys.
	root	A string representing the complete path to the root folder for
		working memory logs.  Default is '~/.wm/logs'
	editor	A string for the file path of the program to edit working
		memory logs.

The configuration file is stored next to the executable file itself by default
but can be changed by providing a WMCFG environment variable.

Provide "search" space separated terms to search the working memory database for.
A table of results that includes all hits will be provided ordered by date.

Usage:
  wm config
  wm search [<term>...]
  wm [<date>]
  wm -h | --help
  wm --version

Options:
  -h --help     Display this screen
  --version     Display the current version`

	opts, err := docopt.ParseArgs(usage, nil, "0.2.0")
	if err != nil {
		log.Fatalln("could not parse arguments:", err)
	}
	var params Parameters
	err = opts.Bind(&params)
	if err != nil {
		log.Fatalln("failed to bind provided parameters: ", err)
	}

	cfgFile := os.Getenv("WMCFG")
	if len(cfgFile) == 0 {
		cfgFile = "wm.toml"
	}

	cfg := GetConfig(cfgFile)

	if params.Config {
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

	if params.Search {
		searchPath := fmt.Sprintf("%s[1-9][0-9][0-9][0-9]/*/*.txt", cfg.Root)
		files, err := filepath.Glob(searchPath)
		if err != nil {
			log.Fatalln("failed to read all files in the root directory: ", err)
		}
		var res []regexp.Regexp
		for _, term := range params.Term {
			re, err := regexp.Compile(term)
			if err != nil {
				log.Fatalln("could not compile search term: ", term)
			}
			res = append(res, *re)
		}
		fmt.Println("searching for", params.Term)
		for _, file := range files {
			fileData, err := os.ReadFile(file)
			if err != nil {
				log.Println(":::note::: failed to read ", file)
			}
			fmt.Println(file, "\n----------\n")
			for _, re := range res {
				locs := re.FindAllIndex(fileData, -1)
				if locs == nil {
					continue
				}
				for i, loc := range locs {
					lb := loc[0] - cfg.ContextSize
					rb := loc[0] + cfg.ContextSize
					if lb < 0 {
						lb = 0
					}
					if rb > len(fileData) {
						rb = len(fileData)
					}
					context := string(fileData[lb:rb])
					contextLines := strings.Split(context, "\n")
					context = ""
					for _, line := range contextLines {
						context += fmt.Sprintf("\t%s\n", line)
					}
					fmt.Println(i+1, ":\n", context)
				}
			}
		}
		os.Exit(0)
	}

	pd, err := parseDateString(params.Date)
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
			_, err = f.WriteString(fmt.Sprintf(`Working Memory File
%d/%d/%d
-------------------

`, pd.month, pd.day, pd.year))
			if err != nil {
				log.Fatalln("working memory file not found at '", wmPath, "'. Created, but failed to write defaults.")
			}

			err = f.Close()
			if err != nil {
				log.Fatalln("failed to close file with error ", err)
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
