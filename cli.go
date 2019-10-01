package cli

import (
	"flag"
	"fmt"
	"os"
	"path"
	"reflect"
	"regexp"
)

// CLI is main CLI application definition. It has a name, description, author
// (which are used only when printing usage syntax), commands and pointers
// to File instances to which standard output or errors are printed (named
// respectively stdout and stderr).
type CLI struct {
	name        string
	desc        string
	author      string
	cmds        map[string]*CLICmd
	parsedFlags map[string]string
	stdout      *os.File
	stderr      *os.File
}

// GetName returns CLI name.
func (c *CLI) GetName() string {
	return c.name
}

// GetDesc returns CLI description.
func (c *CLI) GetDesc() string {
	return c.desc
}

// GetAuthor returns CLI author.
func (c *CLI) GetAuthor() string {
	return c.author
}

// AttachCmd attaches instance of CLICmd to CLI.
func (c *CLI) AttachCmd(cmd *CLICmd) {
	n := reflect.ValueOf(cmd).Elem().FieldByName("name").String()
	if c.cmds == nil {
		c.cmds = make(map[string]*CLICmd)
	}
	c.cmds[n] = cmd
}

// GetCmd returns instance of CLICmd of command k.
func (c *CLI) GetCmd(k string) *CLICmd {
	return c.cmds[k]
}

// GetStdout returns stdout property.
func (c *CLI) GetStdout() *os.File {
	return c.stdout
}

// GetStderr returns stderr property.
func (c *CLI) GetStderr() *os.File {
	return c.stderr
}

// GetCmds returns list of commands.
func (c *CLI) GetCmds() []reflect.Value {
	return reflect.ValueOf(c.cmds).MapKeys()
}

// PrintUsage prints usage information like available commands to CLI stdout.
func (c *CLI) PrintUsage() {
	fmt.Fprintf(c.stdout, c.name+" by "+c.author+"\n"+c.desc+"\n\n")
	fmt.Fprintf(c.stdout, "Available commands:\n")
	for _, i := range c.GetCmds() {
		cmd := c.GetCmd(i.String())
		fmt.Fprintf(c.stdout, path.Base(os.Args[0])+" "+cmd.GetName()+cmd.GetFlagsUsage()+"\n")
	}
}

// AddCmd creates a new command with name n, description d and handler of f.
// It creates instance of CLICmd, attaches it to CLI and returns it.
func (c *CLI) AddCmd(n string, d string, f func(cli *CLI) int) *CLICmd {
	cmd := NewCLICmd(n, d, f)
	c.AttachCmd(cmd)
	return cmd
}

// getFlagSetPtrs creates flagset instance, parses flags and returns list of
// pointers to results of parsing the flags.
func (c *CLI) getFlagSetPtrs(cmd *CLICmd) map[string]interface{} {
	flagSet := flag.NewFlagSet("flagset", flag.ExitOnError)
	flagSetPtrs := make(map[string]interface{})
	flags := cmd.GetFlags()
	for _, i := range flags {
		flagName := i.String()
		flag := cmd.GetFlag(flagName)
		if flag.GetNFlags()&CLIFlagTypeString > 0 ||
			flag.GetNFlags()&CLIFlagTypePathFile > 0 ||
			flag.GetNFlags()&CLIFlagTypeInt > 0 ||
			flag.GetNFlags()&CLIFlagTypeFloat > 0 ||
			flag.GetNFlags()&CLIFlagTypeAlphanumeric > 0 {
			flagSetPtrs[flagName] = flagSet.String(flagName, "", flag.GetDesc())
		} else if flag.GetNFlags()&CLIFlagTypeBool > 0 {
			flagSetPtrs[flagName] = flagSet.Bool(flagName, false, flag.GetDesc())
		}
	}
	flagSet.Parse(os.Args[2:])
	return flagSetPtrs
}

// parseFlags iterates over flags and validates them.
// In case of error it prints out to CLI stderr.
func (c *CLI) parseFlags(cmd *CLICmd) int {
	if c.parsedFlags == nil {
		c.parsedFlags = make(map[string]string)
	}

	flags := cmd.GetFlags()
	flagSetPtrs := c.getFlagSetPtrs(cmd)
	for _, i := range flags {
		flagName := i.String()
		flag := cmd.GetFlag(flagName)
		var flagValue string
		if flag.GetNFlags()&CLIFlagTypeBool == 0 {
			flagValue = *(flagSetPtrs[flagName]).(*string)
		}
		if flag.GetNFlags()&CLIFlagRequired > 0 && (flag.GetNFlags()&CLIFlagTypeString > 0 || flag.GetNFlags()&CLIFlagTypePathFile > 0) {
			if flagValue == "" {
				fmt.Fprintf(c.stderr, "ERROR: Flag --"+flagName+" is missing!\n\n")
				c.PrintUsage()
				return 1
			}
			if flag.GetNFlags()&CLIFlagTypePathFile > 0 && flag.GetNFlags()&CLIFlagMustExist > 0 {
				filePath := flagValue
				if _, err := os.Stat(filePath); os.IsNotExist(err) {
					fmt.Fprintf(c.stderr, "ERROR: File "+filePath+" from --"+flagName+" does not exist!\n\n")
					c.PrintUsage()
					return 1
				}
			}
		}
		if (flag.GetNFlags()&CLIFlagRequired > 0 || flagValue != "") && flag.GetNFlags()&CLIFlagTypeInt > 0 {
			valuePattern := "[0-9]+"
			var reToMatch string
			if flag.GetNFlags()&CLIFlagAllowMany > 0 {
				if flag.GetNFlags()&CLIFlagManySeparatorColon > 0 {
					reToMatch = "^" + valuePattern + "(:" + valuePattern + ")*$"
				} else if flag.GetNFlags()&CLIFlagManySeparatorSemiColon > 0 {
					reToMatch = "^" + valuePattern + "(;" + valuePattern + ")*$"
				} else {
					reToMatch = "^" + valuePattern + "(," + valuePattern + ")*$"
				}
			} else {
				reToMatch = "^" + valuePattern + "$"
			}
			matched, err := regexp.MatchString(reToMatch, flagValue)
			if err != nil || !matched {
				fmt.Fprintf(c.stderr, "ERROR: Flag --"+flagName+" is not a valid integer!\n\n")
				c.PrintUsage()
				return 1
			}
		}
		if (flag.GetNFlags()&CLIFlagRequired > 0 || flagValue != "") && flag.GetNFlags()&CLIFlagTypeFloat > 0 {
			valuePattern := "[0-9]{1,16}\\.[0-9]{1,16}"
			var reToMatch string
			if flag.GetNFlags()&CLIFlagAllowMany > 0 {
				if flag.GetNFlags()&CLIFlagManySeparatorColon > 0 {
					reToMatch = "^" + valuePattern + "(:" + valuePattern + ")*$"
				} else if flag.GetNFlags()&CLIFlagManySeparatorSemiColon > 0 {
					reToMatch = "^" + valuePattern + "(;" + valuePattern + ")*$"
				} else {
					reToMatch = "^" + valuePattern + "(," + valuePattern + ")*$"
				}
			} else {
				reToMatch = "^" + valuePattern + "$"
			}
			matched, err := regexp.MatchString(reToMatch, flagValue)
			if err != nil || !matched {
				fmt.Fprintf(c.stderr, "ERROR: Flag --"+flagName+" is not a valid float!\n\n")
				c.PrintUsage()
				return 1
			}
		}
		if (flag.GetNFlags()&CLIFlagRequired > 0 || flagValue != "") && flag.GetNFlags()&CLIFlagTypeAlphanumeric > 0 {
			var valuePattern string
			if flag.GetNFlags()&CLIFlagAllowUnderscore > 0 && flag.GetNFlags()&CLIFlagAllowDots > 0 {
				valuePattern = "[0-9a-zA-Z_\\.]+"
			} else if flag.GetNFlags()&CLIFlagAllowUnderscore > 0 {
				valuePattern = "[0-9a-zA-Z_]+"
			} else if flag.GetNFlags()&CLIFlagAllowDots > 0 {
				valuePattern = "[0-9a-zA-Z\\.]+"
			} else {
				valuePattern = "[0-9a-zA-Z]+"
			}
			var reToMatch string
			if flag.GetNFlags()&CLIFlagAllowMany > 0 {
				if flag.GetNFlags()&CLIFlagManySeparatorColon > 0 {
					reToMatch = "^" + valuePattern + "(:" + valuePattern + ")*$"
				} else if flag.GetNFlags()&CLIFlagManySeparatorSemiColon > 0 {
					reToMatch = "^" + valuePattern + "(;" + valuePattern + ")*$"
				} else {
					reToMatch = "^" + valuePattern + "(," + valuePattern + ")*$"
				}
			} else {
				reToMatch = "^" + valuePattern + "$"
			}
			matched, err := regexp.MatchString(reToMatch, flagValue)
			if err != nil || !matched {
				fmt.Fprintf(c.stderr, "ERROR: Flag --"+flagName+" is not a valid alphanumeric value!\n\n")
				c.PrintUsage()
				return 1
			}
		}
		if flag.GetNFlags()&CLIFlagTypeString > 0 || flag.GetNFlags()&CLIFlagTypePathFile > 0 {
			c.parsedFlags[flagName] = flagValue
		}
		if flag.GetNFlags()&CLIFlagTypeBool > 0 {
			if *(flagSetPtrs[flagName]).(*bool) == true {
				c.parsedFlags[flagName] = "true"
			} else {
				c.parsedFlags[flagName] = "false"
			}
		}
	}
	return 0
}

// Run parses the arguments, validates them and executes command handler. In
// case of invalid arguments, error is printed to stderr and 1 is returned.
// Return value behaves like exit code.
func (c *CLI) Run(stdout *os.File, stderr *os.File) int {
	c.stdout = stdout
	c.stderr = stderr
	if len(os.Args[1:]) < 1 {
		c.PrintUsage()
		return 1
	}
	for _, cmd := range c.GetCmds() {
		if cmd.String() == os.Args[1] {
			exitCode := c.parseFlags(c.GetCmd(cmd.String()))
			if exitCode > 0 {
				return exitCode
			}
			return c.GetCmd(cmd.String()).Run(c)
		}
	}
	c.PrintUsage()
	return 1
}

// Flag returns value of flag.
func (c *CLI) Flag(n string) string {
	return c.parsedFlags[n]
}

// NewCLI creates new instance of CLI with name n, description d and author a
// and returns it.
func NewCLI(n string, d string, a string) *CLI {
	c := &CLI{name: n, desc: d, author: a}
	return c
}
