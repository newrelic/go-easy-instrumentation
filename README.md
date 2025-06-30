<a href="https://opensource.newrelic.com/oss-category/#new-relic-experimental"><picture><source media="(prefers-color-scheme: dark)" srcset="https://github.com/newrelic/opensource-website/raw/main/src/images/categories/dark/Experimental.png"><source media="(prefers-color-scheme: light)" srcset="https://github.com/newrelic/opensource-website/raw/main/src/images/categories/Experimental.png"><img alt="New Relic Open Source experimental project banner." src="https://github.com/newrelic/opensource-website/raw/main/src/images/categories/Experimental.png"></picture></a>

# Go easy instrumentation [![codecov](https://codecov.io/gh/newrelic/go-easy-instrumentation/graph/badge.svg?token=0qFy6WGpL8)](https://codecov.io/gh/newrelic/go-easy-instrumentation)
Go is a compiled language with an opaque runtime, making it unable to support automatic instrumentation like other languages. For this reason, the New Relic Go agent is designed as an SDK. Since the Go agent is an SDK, it requires more manual work to set up than agents for languages that support automatic instrumentation.

In an effort to make instrumentation easier, the Go agent team created an instrumentation tool that is currently in preview. This tool does most of the work for you by suggesting changes to your source code that instrument your application with the New Relic Go agent.

To get started, check out this four-minute video, or skip down to [How it works](#how-it-works).

[![asciicast](https://asciinema.org/a/r0Il7o2eMiZaLKHIlew3IL2nx.svg)](https://asciinema.org/a/r0Il7o2eMiZaLKHIlew3IL2nx)

## Preview Notice

This feature is currently provided as part of a preview and is subject to our New Relic Experimental policies. Recommended code changes are suggestions only and should be subject to human review for accuracy, applicability, and appropriateness for your environment. This feature should only be used in non-critical, non-production environments that do not contain sensitive data.

This project, its code, and the UX are under heavy development, and should be expected to change. Please take this into consideration when participating in this preview. If you encounter any issues, please report them using Github issues and fill out as much of the issue template as you can so we can improve this tool.

## How it works

This tool doesn't interfere with your application's operation, and it doesn't make any changes to your code directly. Here's what happens when you run the tool:

* It analyzes your code and suggests changes that allow the Go agent to capture telemetry data. 
* You review the changes in the .diff file and decide which changes to add to your source code.

As part of the analysis, this tool may invoke `go get` or other Go language toolchain commands which may modify your `go.mod` file, but not your actual source code.

**IMPORTANT:** This tool can't detect if you already have New Relic instrumentation. Please only use this on applications without any instrumentation.

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
| net/http     | 0.1.0 |
| gRPC         | 0.2.0 |
| Gin          | 0.4.0 |

## Installation

Before you start the installation steps below, make sure you have a version of Go installed that is within the support window for the current [Go programming language lifecycle](https://endoflife.date/go).

Installation Steps have been moved to: https://docs.newrelic.com/docs/apm/agents/go-agent/installation/install-automation-new-relic-go/#go-easy-install

### Building from source for development

1. Clone this repository to a directory on your system. For example:
    ```sh
    git clone https://github.com/newrelic/go-easy-instrumentation.git
    ```
2. Go into that directory:
    ```sh
    cd go-easy-instrumentation
    ```
3. Resolve any third-party dependencies:
    ```sh
    go mod tidy
    ```
4. Build and run from the root of the repo:
    ```
    go run . instrument $MY_APP
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
This is an experimental product, and New Relic is not offering official support at the moment. Please create issues in Github if you are encountering a problem that you're unable to resolve. When creating issues, its vital that you include as much of the requested information as possible. This enables us to get to the root cause of the issue much more quickly. Please also make sure to search existing issues before creating a new one.

## Contributing
We encourage your contributions! Keep in mind when you submit your pull request, you'll need to sign the CLA via the click-through using CLA-Assistant. You only have to sign the CLA one time per project.
If you have any questions, or to execute our corporate CLA, required if your contribution is on behalf of a company, please drop us an email at opensource@newrelic.com.

To learn more about how to contribute features to this project, read the [parser readme](/parser/README.md). The [godoc](https://pkg.go.dev/github.com/newrelic/go-easy-instrumentation) for this project is also kept up to date, and can be a great resource.

## License
Go easy instrumentation is licensed under the [Apache 2.0](http://apache.org/licenses/LICENSE-2.0.txt) License.
>This tool also uses source code from third-party libraries. You can find full details on which libraries are used and the terms under which they are licensed in the third-party notices document.
