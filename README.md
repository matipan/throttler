# throttler

`Throttler` is a package that implements a simple throttling algorithm based on CPU usage.

# Algorithm

Within the throttler we define the following parameters:

* L=Limit CPU Usage
* X=CPU Usage
* R=% of allowed requests
* K=multiplier for the step difference
* S=K*(L-X) -> step to increase/decrease
* T=interval
* ST=step interval

Every `ST` we will collect CPU usage information and store it. After T ends we compute the average CPU usage `(X)` and evaluate what action is necessary:

* `IF X >= L` 	-> reduce R by substracting S, rounding at 0
* `IF X < L` 	-> increase R by adding S, rounding at 100

A user of T will simply call `t.Start` so that the throttler starts collecting CPU statistics. Every request/event the user will call `t.Allow` to ask if the request is allowed to go through or if it needs to be throttled.

# Tests

## Linear throttle

## Constant throttle

## Random noisy throttling
