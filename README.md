<a href="https://opensource.newrelic.com/oss-category/#new-relic-experimental"><picture><source media="(prefers-color-scheme: dark)" srcset="https://github.com/newrelic/opensource-website/raw/main/src/images/categories/dark/Experimental.png"><source media="(prefers-color-scheme: light)" srcset="https://github.com/newrelic/opensource-website/raw/main/src/images/categories/Experimental.png"><img alt="New Relic Open Source experimental project banner." src="https://github.com/newrelic/opensource-website/raw/main/src/images/categories/Experimental.png"></picture></a>

# Go instrumentation tool
Go is a compiled language with an opaque runtime, making it unable to support automatic instrumentation like other languages. For this reason, the New Relic Go agent is designed as an SDK. Since the Go agent is an SDK, it requires more manual work to set up than agents for languages that support automatic instrumentation.

In an effort to reduce that manual effort, the Go agent team created an easy instrumentation tool for Go that is currently in preview. This tool will do most of the work for you by suggesting changes to your source code that instrument your application with the New Relic Go agent.

To get started, check out this four-minute video, or skip down to [How it works](#how-it-works).

[![asciicast](https://asciinema.org/a/r0Il7o2eMiZaLKHIlew3IL2nx.svg)](https://asciinema.org/a/r0Il7o2eMiZaLKHIlew3IL2nx)

## Preview Notice

This feature is currently provided as part of a preview and is subject to our New Relic Experimental policies. Recommended code changes are suggestions only and should be subject to human review for accuracy, applicability, and appropriateness for your environment. This feature should only be used in non-critical, non-production environments that do not contain sensitive data.

This project, its code, and the UX are under heavy development, and should be expected to change. Please take this into consideration when participating in this preview. If you encounter any issues, please report them using Github issues and fill out as much of the issue template as you can so we can improve this tool.

## How it works

This tool will not interfere with your application's operation, and it doesn't make any changes to your code directly. Here's what happens:

* It analyzes your source code and identifies opportunities to instrument it.
* It suggests changes to your code that use the New Relic Go agent SDK to capture telemetry data.
* You review those sugestions that are inserted in a `.diff` file and decide which changes to apply to your source code.

As part of the analysis, this tool may invoke `go get` or other Go language toolchain commands which may modify your `go.mod` file, but not your actual source code.

**IMPORTANT:** This tool can't detect if you already have New Relic instrumentation. Please only use this on applications without any instrumentation.

## What is instrumented?

The scope of what this tool can instrument in your application is limited to these actions:

 - Capturing errors in any function wrapped or traced by a transaction
 - Tracing locally defined functions that are invoked in the application's `main()` method with a transaction
 - Tracing async functions and function literals with an async segment
 - Wrapping HTTP handlers
 - Injecting distributed tracing into external traffic

**ONLY** the following Go packages and libraries are currently supported:

  - standard library
  - net/http

## Installation

Before you start the installation steps below, make sure you have a version of Go installed that is within the support window for the current [Go programming language lifecycle](https://endoflife.date/go).

1. Clone this repository to a directory on your system. For example:
        ```sh
        git clone .../go-agent-pre-instrumentation
        ```
2. Go into that directory:
        ```sh
        cd go-agent-pre-instrumentation
        ```
3. Resolve any third-party dependencies:
        ```sh
        go mod tidy
        ```
## Generate instrumentation suggestions

For detailed instructions on how to generate instrumentation suggestions, see our documentation at [docs.newrelic.com](https://docs/apm/agents/go-agent/installation/install-automation-new-relic-go/#generate-suggestions).

## Support
This is an experimental product, and New Relic is not offering official support at the moment. Please create issues in Github if you are encountering a problem that you're unable to resolve. When creating issues, its vital to include as much of the prompted for information as possible. This enables us to get to the root cause of the issue much more quickly. Please also make sure to search existing issues before creating a new one.

## Contributing
We encourage your contributions to improve the Go instrumentation tool! Keep in mind when you submit your pull request, you'll need to sign the CLA via the click-through using CLA-Assistant. You only have to sign the CLA one time per project.
If you have any questions, or to execute our corporate CLA, required if your contribution is on behalf of a company, please drop us an email at opensource@newrelic.com.


## License
This Go instrumentation tool is licensed under the [Apache 2.0](http://apache.org/licenses/LICENSE-2.0.txt) License.
>The Go instrumentation tool also uses source code from third-party libraries. You can find full details on which libraries are used and the terms under which they are licensed in the third-party notices document.
