package httpagg

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"regexp"
	"strings"
	"text/template"
	"time"

	"github.com/montanaflynn/stats"
	"go.k6.io/k6/js/modules"
	"go.k6.io/k6/js/modules/k6/http"
)

func init() {
	modules.Register("k6/x/httpagg", new(Httpagg))
}

// Httpagg is the k6 extension
type Httpagg struct{}

type options struct {
	FileName       string `js:"fileName"`
	AggregateLevel string `js:"aggregateLevel"`
}

// metrics data of single object
type HttpObjectMetrics struct {
	FailedRequest   float64
	ServerError     float64
	MinDuration     float64
	AverageDuration float64
	P99Duration     float64
	MaxDuration     float64
}

// single http object is 1 pattern on 1 method
type HttpObject struct {
	UrlPattern string
	HttpMethod string
}

// filtering http response data to only essentials
type HttpResponseFiltered struct {
	Url      string
	Status   int
	Method   string
	Duration float64
}

func AppendJSONToFile(fileName string, jsonData HttpResponseFiltered) {
	f, err := os.OpenFile(fileName, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0666)
	check(err)
	defer f.Close()

	file, _ := json.MarshalIndent(jsonData, "", " ")
	falseContent, err := f.Write(file)
	check(err)

	if false {
		fmt.Println(falseContent)
	}
}

func trimBaseURL(url string) string {
	// Define a regex pattern to match the word that ends with ".com"
	baseURLPattern := `([^\s]+\.com)`

	// Find the first match using regex
	regex := regexp.MustCompile(baseURLPattern)
	match := regex.FindString(url)

	// Trim the word that ends with ".com" and replace with "{BASE_URL}"
	trimmedURL := strings.Replace(url, match, "{BASEURL}", 1)

	return trimmedURL
}

func replaceGUIDs(url string) string {
	// Replace GUID-like strings with "{GUID}"
	guidPattern := `[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`
	regexGUID := regexp.MustCompile(guidPattern)
	trimmedURL := regexGUID.ReplaceAllString(url, "{GUID}")

	return trimmedURL
}

func trimAndReplaceURL(url string) string {
	// Trim the word that ends with ".com" and replace with "{BASE_URL}"
	baseURL := trimBaseURL(url)
	// Replace GUID within URL with "{GUID}" text
	trimmedURL := replaceGUIDs(baseURL)

	return trimmedURL
}

func filterHttpResponse(response http.Response) HttpResponseFiltered {
	return HttpResponseFiltered{
		Url:      response.Request.URL,
		Status:   response.Status,
		Method:   response.Request.Method,
		Duration: response.Timings.Duration,
	}
}

func getJSONAggrResults(fileName string) map[HttpObject][]HttpResponseFiltered {
	jsonFile, err := os.Open(fileName)
	if err != nil {
		fmt.Println("[httpagg] The result file named " + fileName + " does not exist")
		var responsesMap = make(map[HttpObject][]HttpResponseFiltered)
		return responsesMap
	}

	var responsesMap = make(map[HttpObject][]HttpResponseFiltered)
	byteValue, _ := io.ReadAll(jsonFile)
	responsesCoded := json.NewDecoder(strings.NewReader(string(byteValue[:])))

	for {
		var response HttpResponseFiltered
		var pattern string
		var currentHttpObject HttpObject
		err := responsesCoded.Decode(&response)
		if err == io.EOF {
			// all done
			break
		}

		check(err)

		// get patterns from request URL
		pattern = trimAndReplaceURL(response.Url)
		currentHttpObject = HttpObject{
			UrlPattern: pattern,
			HttpMethod: response.Method,
		}

		// Check if the URL pattern + method exists in the map
		if existingPattern, found := responsesMap[currentHttpObject]; found {
			// Append the response to the existing data
			existingPattern = append(existingPattern, response)

			// Update the map with the modified row
			responsesMap[currentHttpObject] = existingPattern
		} else {
			// URL pattern + method combo not found, add it to map with its first response
			responsesMap[currentHttpObject] = []HttpResponseFiltered{response}
		}
	}
	return responsesMap
}

func check(e error) {
	if e != nil {
		panic(e)
	}
}

// Custom function must have only 1 return value, or 1 return value and an error
func formatDate(timeStamp time.Time) string {
	// Define layout for formatting timestamp to string
	// return timeStamp.Format("01-02-2006")
	return timeStamp.Format("Mon, 02 Jan 2006")

}

// Map name formatDate to formatDate function above
var funcMap = template.FuncMap{
	"formatDate": formatDate,
}

func (*Httpagg) CheckRequest(response http.Response, status bool, options options) {
	if options.FileName == "" {
		options.FileName = "httpagg.json"
	}

	if options.AggregateLevel == "" {
		options.AggregateLevel = "onError"
	}

	switch options.AggregateLevel {
	case "onError":
		if !status {
			AppendJSONToFile(options.FileName, filterHttpResponse(response))
		}
	case "onSuccess":
		if status {
			AppendJSONToFile(options.FileName, filterHttpResponse(response))
		}
	case "all":
		AppendJSONToFile(options.FileName, filterHttpResponse(response))
	default:
		// by default, aggregate only invalid http responses
		if !status {
			AppendJSONToFile(options.FileName, filterHttpResponse(response))
		}
	}
}

func (*Httpagg) GenerateRaport(httpaggResultsFileName string, httpaggReportFileName string) {
	const tpl = `
	<html lang="en">

<head>
    <meta charset="utf-8" />
    <title>MR API Performance Report</title>
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <link rel="icon" href="https://medirecords.com/wp-content/uploads/2020/02/logo.svg" type="image/x-icon">
    <link rel="stylesheet" href="/css/demo.css" />
    <link rel="preconnect" href="https://fonts.gstatic.com" />
    <link rel="stylesheet" href="https://fonts.googleapis.com/css2?family=Inter&family=Source+Code+Pro&display=swap" />
    <script src="https://code.jquery.com/jquery-3.5.1.js"></script>
    <script src="https://cdn.datatables.net/1.12.1/js/jquery.dataTables.min.js"></script>
    <style>
        .container {
            width: 96%;
            min-width: 30%;
            max-height: 100%;
            border: 1px solid #ece8f1;
            padding: 2%;
            font-family: Helvetica, sans-serif;
        }

        table {
            color: #3c3c64;
            font-size: 14px;
            line-height: 16px;
            border-collapse: collapse;
            width: 100%;
            max-height: 50px;
            border: 1px solid #ece8f1;
        }

        th {
            background: #949494;
            color: white;
            font-size: 11px;
            line-height: 16px;
            padding: 10px 16px;
            text-align: left;
            text-transform: uppercase;
            box-sizing: border-box;
            border-collapse: collapse;
            cursor: pointer;
            white-space: nowrap;
        }

        td {
            padding: 20px;
            vertical-align: baseline;
            border-bottom: 1px solid #ece8f1;
            box-sizing: border-box;
        }

        tr {
            cursor: pointer;
        }

        a {
            color: #6cbc28;
            cursor: pointer;
            font-weight: 500;
            padding-bottom: 1px;
            position: relative;
            text-decoration: none;
            transition: all .3s;
            outline-color: #00cdff;
            background-color: transparent;
            box-sizing: border-box;
            font-size: 1em;
            line-height: 25px;
            border-collapse: collapse;
        }

        h2 {
            font-size: 25px;
            font-weight: 400;
            line-height: 35px;
            margin-top: 50px;
            margin-bottom: 15px;
            position: relative;
            box-sizing: border-box;
            color: #3c3c64;
            font-family: Helvetica, sans-serif;
        }

        input {
            color: #5a5c87;
            font-weight: 400;
            appearance: none;
            border: 1px solid #5a5c87;
            border-radius: 0;
            box-shadow: 0 1px 5px rgba(60, 60, 100, .05);
            color: #3c3c64;
            flex: 1 1;
            font-size: 15px;
            font-weight: 500;
            line-height: 20px;
            outline: none;
            overflow-x: auto;
            padding: 0 40px 0 15px;
            text-align: left;
            overflow: visible;
            font-family: inherit;
            margin: 0;
            box-sizing: border-box;
            width: 100%;
            padding: 12px;
            padding-left: 20px;
            margin-bottom: 30px;
            margin-top: 20px;
            border-radius: 8px;
        }

        select {
            float: right;
            border-style: none;
            background-color: transparent;
            border: none;
            color: black;
            cursor: pointer;
            font-size: 12px;
            font-weight: 700;
            position: relative;
            transition: color .3s ease;
            text-transform: none;
            overflow: visible;
            line-height: 1.15;
            margin-right: fill;
            align-items: center;
            display: flex;
            flex-direction: column;
            position: relative;
            padding-right: 5px;
            font-size: 14px;
            border-color: blue;
            position: relative;
            -moz-appearance: none;
            -webkit-appearance: none;
            appearance: none;
            border: none;
            background: white url("data:image/svg+xml;utf8,<svg width='10' height='10' viewBox='0 0 10 10' fill='none' xmlns='http://www.w3.org/2000/svg' ><path d='M9 3 5 7 1 3' stroke='black' stroke-width='1.6'></path></svg>") no-repeat;
            background-position: right 0px top 50%;
            font-family: Helvetica, sans-serif;
        }

        .dataTables_info {
            margin-top: 30px;
            color: #3c3c64;
            font-size: 14px;
            line-height: 25px;
            box-sizing: border-box;
            margin-bottom: 20px;
            
        }

        #example_paginate {
            display: flex;
        }

        #example_previous {
            align-items: flex-start;
            margin-right: auto;
            padding-left: 0px;
            padding-right: 0;
            color: #6cbc28;
            font-size: 12px;
            font-weight: 700;
            line-height: 18px;
            text-transform: uppercase;
            cursor: pointer;
            display: flex;
            flex-direction: column;
            position: relative;
            text-decoration: none;
            
        }

        #example_next {
            align-items: flex-start;
            margin-left: auto;
            padding-left: 0px;
            padding-right: 0px;
            color: #6cbc28;
            font-size: 12px;
            font-weight: 700;
            line-height: 18px;
            text-transform: uppercase;
            cursor: pointer;
            display: flex;
            flex-direction: column;
            position: relative;
            text-decoration: none;
        }

        .paginate_button {
            margin-left: 2px;
            margin-right: 2px;
            padding-left: 0px;
            padding-right: 0px;
            color: #6cbc28;
            font-size: 13px;
            font-weight: 700;
            line-height: 18px;
            text-transform: uppercase;
            cursor: pointer;
            flex-direction: column;
            position: relative;
            text-decoration: none;
        }

        .paginate_button.current {
            color: #6cbc28;
        }

        textarea {
            color: white;
            background-color: #3c3c64;
            resize: none;
            width: 100%;
            border: none;
            outline: none;
        }

        code {
            line-height: 16px;
        }

        #example_filter {
            padding-top: 40px;
        }
        
        #example_wrapper {
            padding-top: 30px;
        }

        .bold-text {
            font-weight: bold;
        }

        .green-row {
            background-color: #efe;
            color: #080
        }

        .red-row {
            background-color: #fee;
            color: #800
        }

        .yellow-row {
            background-color: #ffe;
            color: #880
        }

        .blue-row {
            background-color: #eef;
            color: #008
        }

        .purple-row {
            background-color: #fef;
            color: #808
        }
    </style>
</head>

<body>
    <div class="container">
        <table id="example">
            <thead>
                <tr>
                    <th rowspan="2" colspan="1">METHOD</th>
                    <th rowspan="2">URL</th>
                    <th colspan="3">TOTAL REQUEST</th>
                    <th colspan="4">DURATION (millisecond)</th>
                </tr>
                <tr>
                    <th>Total</th>
                    <th>Failed</th>
                    <th>500 Error</th>
                    <th>MIN</th>
                    <th>AVG</th>
                    <th>MAX</th>
                    <th>P(99.99)</th>
                </tr>
            </thead>
            <tbody>
                {{ range $key, $value := . }}
                    <tr>
                        <td class="bold-text">{{$key.HttpMethod}}</td>
                        <td>{{$key.UrlPattern}}</td>
                        <td>{{len $value}}</td>
                        {{ $var := processHttpDuration $value }}
                        {{ $resp := $value }}
                            <td>{{$var.FailedRequest}}</td>
                            <td>{{$var.ServerError}}</td>
                            <td>{{$var.MinDuration}}</td>
                            <td>{{$var.AverageDuration}}</td>
                            <td>{{$var.MaxDuration}}</td>
                            <td class="bold-text">{{$var.P99Duration}}</td>
                        </tr>
                    {{ end }}
            </tbody>
        </table>
    </div>

    <script type="module">
        $(document).ready(function () {
            $('#example').DataTable({
                "language": {
                    "lengthMenu": '_MENU_',
                    "search": '<i class="search"></i>',
                    "searchPlaceholder": "Search Every Text...",
                },
                order: []
            });

            // Function to determine row color based on data
            function determineRowColor(rowData) {
                // Change row color based on the method value
                if (rowData[0] === 'GET') {
                    return 'green-row';
                } else if (rowData[0] === 'POST') {
                    return 'yellow-row';
                } else if (rowData[0] === 'PUT') {
                    return 'blue-row';
                } else if (rowData[0] === 'DELETE') {
                    return 'red-row';
                } else if (rowData[0] === 'PATCH'){
                    return 'purple-row';
                } else {
                    return ''
                }
            }

            // Apply row colors
            function applyRowColors() {
                $('#example tbody tr').each(function () {
                    const rowData = $('#example').DataTable().row(this).data();
                    const rowColorClass = determineRowColor(rowData);
                    $(this).addClass(rowColorClass);
                });
            }
            
            // Apply row colors when table is initially loaded
            applyRowColors();
            
            // Reapply row colors on each draw event (including pagination)
            $('#example').on('draw.dt', function () {
                applyRowColors();
            });

            // Trigger row color assignment on initial load
            $('#example tbody').trigger('draw');

            // change testform
            document.querySelectorAll("textarea").forEach(element => {
                function autoResize(el) {
                    el.style.height = el.scrollHeight + 'px';
                }
                autoResize(element);
                element.addEventListener('input', () => autoResize(element));    
            });

            $(document).on("click", 'thead tr', function() {
                $('table tr').eq(1).trigger('click');
            });

            $(document).on("click", '.paginate_button', function() {
                $('table tr').eq(1).trigger('click');
            });
        });
    </script>
</body>

</html>
	`

	// temp := template.Must(template.New("index.txt").Funcs(funcMap).ParseFiles("index.txt"))
	var responsesMap = getJSONAggrResults(httpaggResultsFileName)
	temp, err := template.New("index.txt").Funcs(template.FuncMap{
		"processHttpDuration": func(arrResponse []HttpResponseFiltered) HttpObjectMetrics {
			var err500, fail int
			var tmp = make([]float64, 0, len(arrResponse))
			for _, element := range arrResponse {
				if element.Status >= 500 {
					err500 += 1
				} else if (element.Status == 0) || (element.Status >= 400 && element.Status < 500) {
					fail += 1
				}
				tmp = append(tmp, element.Duration)
			}
			min, _ := stats.Min(tmp)
			avg, _ := stats.Mean(tmp)
			p99, _ := stats.Percentile(tmp, 99.99)
			max, _ := stats.Max(tmp)
			return HttpObjectMetrics{
				FailedRequest:   float64(fail),
				ServerError:     float64(err500),
				MinDuration:     math.Round(min*100) / 100,
				AverageDuration: math.Round(avg*100) / 100,
				P99Duration:     math.Round(p99*100) / 100,
				MaxDuration:     math.Round(max*100) / 100,
			}
		},
	}).Parse(tpl)
	check(err)

	if httpaggResultsFileName == "" {
		httpaggResultsFileName = "httpagg.json"
	}

	if httpaggReportFileName == "" {
		httpaggReportFileName = "httpaggReport.html"
	}

	if len(responsesMap) != 0 {
		file, err := os.Create(httpaggReportFileName)
		check(err)

		err = temp.Execute(file, responsesMap)
		check(err)
	}
}

var index string
