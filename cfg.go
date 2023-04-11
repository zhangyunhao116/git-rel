package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/zhangyunhao116/skipmap"
)

const configpath = ".git/gitrel.cfg"

type config struct {
	// Actual data in the file is:
	// BRANCH_NAME1
	// INDEX_COMIT1
	// BRANCH_NAME2
	// INDEX_COMIT2
	// ...
	data *skipmap.StringMap[string]
}

func NewConfig() *config {
	c := &config{data: skipmap.NewString[string]()}
	_, err := os.Stat(configpath)
	if err != nil {
		// No config file.
		if os.IsNotExist(err) {
			return c
		}
		panic(err)
	}
	rawdata, err := os.ReadFile(configpath)
	if err != nil {
		panic(err)
	}
	data := strings.Split(string(rawdata), "\n")
	if len(data)%2 != 0 {
		panic("invalid data len" + fmt.Sprintf(" %d ", len(data)/2))
	}
	for i := 0; i < len(data)/2; i++ {
		c.data.Store(data[i*2], data[i*2+1])
	}
	return c
}

func (c *config) Get(branch string) (commit string, ok bool) {
	return c.data.Load(branch)
}

func (c *config) Add(branch string, commit string) {
	c.data.Store(branch, commit)
}

func (c *config) Save() {
	var file *os.File
	_, err := os.Stat(configpath)
	if err != nil {
		if os.IsNotExist(err) {
			// No config file.
			file, err = os.Create(configpath)
			if err != nil {
				panic(err)
			}
		} else {
			panic(err)
		}
	} else {
		file, err = os.OpenFile(configpath, os.O_RDWR, 0644)
		if err != nil {
			panic(err)
		}
	}

	var data []string
	c.data.Range(func(key, value string) bool {
		data = append(data, key, value)
		return true
	})
	file.WriteString(strings.Join(data, "\n"))
	file.Close()
}
