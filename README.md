# xk6-httpagg
[k6](https://github.com/grafana/k6) extension that allows you to aggregate the results of HTTP requests and view them one by one using a generated `.html` report. Until now it was only possible to analyze them using [WDP (Web Debugging Proxy)](https://k6.io/blog/k6-load-testing-debugging-using-a-web-proxy/). Implemented using the [xk6](https://github.com/grafana/xk6) system.

## Build
```shell
xk6 build v0.38.3 --with github.com/gpiechnik2/xk6-httpagg@latest
```
                                             
## Example
```javascript
import httpagg from 'k6/x/httpagg';


export default function () {
  const response = http.get('http://httpbin.test.k6.io/endpointThatWillReturn404Error');
  const status = check(
    res,
    {
      'response code was 200': (res) => res.status == 200
    }
  ); // the status variable will be false because the assertion inside does not match

  httpagg.checkRequest(
    response = response,
    status = status,
    fileName = "myFilenameWithRequestsAggregated.json",
    aggregateLevel = "onSuccess" // response with the request will not be aggregated because 
    // we set the aggregation level to "onSuccess". The default level is "onError", which is 
    // when any of the assertions from the k6 "check" function fails and the entire function 
    // returns false
  );

  // or (without the optional fields)
  httpagg.checkRequest(response, status); // this request & response will be aggregated because 
  // we have not set the aggregation level and the default "onError" will be used. Additionally, 
  // a file will be created with the default name "httpagg.json"

  // or
  // IMPORTANT: We can use the "all" aggregation level to aggregate all requests regardless of 
  // the check result
  httpagg.checkRequest(
    response = response,
    status = status,
    aggregateLevel = "all"
  );
}

export function teardown(data) {
    httpagg.generateRaport("myFilenameWithRequestsAggregated.json", "myHtmlReport.html")

    // or (without the optional fields)
    httpagg.generateRaport("myFilenameWithRequestsAggregated.json") // the default name of the 
    // html report that will be created is "httpaggReport.html"
}
```

## Report view
<img src="exampleResultsView.png">

## Run sample script
```shell
./k6 run ./script.js
```
