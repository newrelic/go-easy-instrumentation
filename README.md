[![Community Plus header](https://github.com/newrelic/opensource-website/raw/main/src/images/categories/Community_Plus.png)](https://opensource.newrelic.com/oss-category/#community-plus)

# Go Easy Instrumentation [![GoDoc](https://godoc.org/github.com/newrelic/go-easy-instrumentation?status.svg)](https://godoc.org/github.com/newrelic/go-easy-instrumentation) [![Go Report Card](https://goreportcard.com/badge/github.com/newrelic/go-easy-instrumentation)](https://goreportcard.com/report/github.com/newrelic/go-easy-instrumentation) [![codecov](https://codecov.io/gh/newrelic/go-easy-instrumentation/graph/badge.svg?token=0qFy6WGpL8)](https://codecov.io/gh/newrelic/go-easy-instrumentation)
Go is a compiled language with an opaque runtime, making it unable to support automatic instrumentation like other languages. For this reason, the New Relic Go agent is designed as an SDK. Since the Go agent is an SDK, it requires more manual work to set up than agents for languages that support automatic instrumentation.

In an effort to make instrumentation easier, the Go agent team created an instrumentation tool. This tool does most of the work for you by suggesting changes to your source code that instrument your application with the New Relic Go agent.

## Quick Start

### Install

```sh
go install github.com/newrelic/go-easy-instrumentation@latest
```

Make sure `$GOPATH/bin` (or `$HOME/go/bin` by default) is in your `PATH`.

### Use

From your application directory:
```sh
cd /path/to/your/app
go-easy-instrumentation instrument .
git apply new-relic-instrumentation.diff
```

Or specify a path from anywhere:
```sh
go-easy-instrumentation instrument /path/to/your/app
git apply /path/to/your/app/new-relic-instrumentation.diff
```

<details>
<summary><b>Alternative: Build from source</b></summary>

```sh
git clone https://github.com/newrelic/go-easy-instrumentation.git
cd go-easy-instrumentation
go build -o go-easy-instrumentation .
sudo mv go-easy-instrumentation /usr/local/bin/   # or anywhere on your PATH
```
</details>

### CLI Flags

| Flag | Short | Description |
| ---- | ----- | ----------- |
| `--debug` | `-d` | Enable debug logging with text-mode output (no TUI) |
| `--exclude` | `-e` | Comma-separated list of folders to exclude |
| `--output` | `-o` | Custom diff output file path (must be `.diff`) |

```sh
go-easy-instrumentation instrument --debug /path/to/your/app
go-easy-instrumentation instrument --exclude "vendor,testdata" /path/to/your/app
go-easy-instrumentation instrument --output /tmp/changes.diff /path/to/your/app
```

> **Note:** In non-TTY environments (CI/CD, Docker, piped output), the tool automatically uses text-mode output.

### Interactive Mode

Run without a subcommand to scan the current directory and choose which files to instrument:
```sh
go-easy-instrumentation
go-easy-instrumentation --exclude "vendor,end-to-end-tests"
```

## Development

### Running from source
```sh
go run . instrument /path/to/your/app
```


## How it works

This tool doesn't interfere with your application's operation, and it doesn't make any changes to your code directly. Here's what happens when you run the tool:

* It analyzes your code and suggests changes that allow the Go agent to capture telemetry data. 
* It captures any existing transactions in your application and doesn't add additional transactions.
* You review the changes in the .diff file and decide which changes to add to your source code.

As part of the analysis, this tool may invoke `go get` or other Go language toolchain commands which may modify your `go.mod` file, but not your actual source code.


## What is instrumented?

The scope of what this tool can instrument in your application is limited to these actions:

 - A best effort to capture errors at the root cause
 - Tracing locally defined synchronous functions that are invoked in the application's `main()` method with a transaction. Note that we will not attempt to trace async code in the main method due to issues of complexity, and will instead prompt you to manually instrument this code at your own discretion.
 - Starting tracing from entrypoints into your application with instrumentation from one of the supported libraries
 - Injecting distributed tracing into external traffic with one of the supported libraries

## Supported Libraries
The following libraries are supported for automatic instrumentation. Listed below are the minimum version of this tool needed to support each library, however it is always recommended that you upgrade to the latest version of this tool since there are a number of improvements in instrumentation coverage and quality that you would otherwise miss out on.

| Library Name | Minimum Version |
| ------------ | --------- |
| net/http     | v1.0.0 |
| gRPC         | v1.0.0 |
| Gin          | v1.0.0 |
| Go-chi       | v1.0.0 |
| mysql        | v1.0.0 |
| slog         | v1.0.0 |



### Updating an existing version of Go Easy Instrumentation
Updating or Downgrading Go Easy Instrumentation is a straightforward process.

 1. Check for new [tagged releases](https://github.com/newrelic/go-easy-instrumentation/tags) OR run the following command within the project folder:
    ```
    git tag
    ```
 2. Change the tag version:
    ```
    git checkout <tag_name>
    ```


### Manually Run Unit Tests
Unit tests can be run from the root of this repository with this command:
```sh
go test ./...
```

### Manually Run End To End Suite
The end to end suite is designed to run easily on a developer's local machine. To run this suite, run this command from the root directory of this repository:
```sh
./end-to-end-tests/testrunner
```

To modify which tests get run, modify the `end-to-end-tests/testcases.json` file in your local development environment. 

## Generate instrumentation suggestions

For detailed instructions on how to generate instrumentation suggestions, see our documentation at [docs.newrelic.com](https://docs.newrelic.com/docs/apm/agents/go-agent/installation/install-automation-new-relic-go/#generate-suggestions).

## Support
Should you need assistance with New Relic products, you are in good hands with several support channels.  

If the issue has been confirmed as a bug or is a Feature request, please file a Github issue.


* [New Relic Documentation](https://docs.newrelic.com/docs/apm/agents/go-agent/installation/install-automation-new-relic-go/#generate-suggestions): Comprehensive guidance for using our platform
* [Troubleshooting framework](https://discuss.newrelic.com/t/troubleshooting-frameworks/108787): Steps you through common troubleshooting questions
* [New Relic Community](https://discuss.newrelic.com/tags/goagent): The best place to engage in troubleshooting questions
* [New Relic Developer](https://developer.newrelic.com/): Resources for building a custom observability applications
* [New Relic University](https://learn.newrelic.com/): A range of online training for New Relic users of every level


## Contributing
We encourage your contributions! Keep in mind when you submit your pull request, you'll need to sign the CLA via the click-through using CLA-Assistant. You only have to sign the CLA one time per project.
If you have any questions, or to execute our corporate CLA, required if your contribution is on behalf of a company, please drop us an email at opensource@newrelic.com.

To learn more about how to contribute features to this project, read the [parser readme](/parser/README.md). The [godoc](https://pkg.go.dev/github.com/newrelic/go-easy-instrumentation) for this project is also kept up to date, and can be a great resource.

## License
Go easy instrumentation is licensed under the [Apache 2.0](http://apache.org/licenses/LICENSE-2.0.txt) License.
>This tool also uses source code from third-party libraries. You can find full details on which libraries are used and the terms under which they are licensed in the third-party notices document.
