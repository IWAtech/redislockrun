# redislockrun

Makes sure that only one instance of a command runs in the cluster.

## Usage

	redislockrun [OPTIONS] COMMAND

All arguments after the flags, or after the argument `--`, are
interpreted as a command to be invoked by the system.

For example:

	redislockrun -- sleep 10
	redislockrun -timeout=15m -- sleep 10

## Configuration

Configuration is done with environment variables:

* `REDISLOCKRUN_ADDR`: Address of Redis server
* `REDISLOCKRUN_PASSWORD`: Redis Password
* `REDISLOCKRUN_DB`: Database number

To prevent deadlocks, locks expire after 30 minutes by default. 
This should be adapted depending on the workload, and should be higher
than the expected run time of the command. 

This can be changed by passing the `-timeout` flag to `redislockrun`.
