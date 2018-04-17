# go-skeleton

## Overview

This repository contains the template for all our go projects.

The goal of this repository is to provide a template that can be used for all our future Go projects. This helps to setup all projects in the same manner.

## Testing
For testing we use table driven unit tests. The idea is very simple: you create a list of structs with the params and the expected return values.

The actual test consists only of a loop going through the list and comparing the


## Config
A config file can be specified with `--config=config-file.yaml`. See `config.yaml` for an example. The library [`viper`](https://github.com/spf13/viper) for Go is used for configuration. So every configuration can be either specified in the config file or via environemnt variable.

## Dependencies
We use:

* Viper: github.com/spf13/viper (Configuration framework)
* Cobra: github.com/spf13/cobra (CLI framework)

### Management
We manage our dependencies with the golang dep tool. The golang dep tool saves all the dependencies to the vendor folder.

The vendor folder is in the .gitignore file per default to reduce the size of the repositories.

To get the dependencies after checkout:

````
dep ensure -v
````

## Build
`go build`
