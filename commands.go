package main

import "errors"

type command struct {
	Name string
	Args []string
}

type commands struct {
	registeredCommands map[string]func(*AppState, command) error
}

func (c *commands) register(name string, f func(*AppState, command) error) {
	c.registeredCommands[name] = f
}

func (c *commands) run(s *AppState, cmd command) error {
	f, ok := c.registeredCommands[cmd.Name]
	if !ok {
		return errors.New("command not found")
	}
	return f(s, cmd)
}
