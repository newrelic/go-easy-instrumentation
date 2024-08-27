# Testing package
This very simple testing harness is used to instrument applications using various
runtime parameters, checking the diff file output against a known-good case.

To invoke it, run
```
$ ./testrunner testcases.json
```
It exits with a nonzero status if any of the test cases failed. It also prints error information
about failed tests to its standard output.

## Configuration
The test cases are defined in the JSON file supplied to `testrunner` as its argument. 

Please create a reference diff file in the test directory named "expect.ref" rather than utilize the cmp field. This helps us reduce tool and filesystem clutter.

This is an object containing a single field:

`tests` -- A list of objects, each of which defines the parameters of a test case.

Each test case is a JSON object with these fields:

`name` -- If present, override the default application name in the instrumented application

`dir` -- The directory (absolute or relative to the `parser` directory) where the instrumented application can be found.

`cmp` -- Optional; The name of the reference file which should match the generated diff output. This tool will look for a file named "expect.ref" in the test "dir" by default.
