package httpagg

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"os"
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

type HttpObject struct {
	FailedRequest   float64
	ServerError     float64
	MinDuration     float64
	AverageDuration float64
	P99Duration     float64
	MaxDuration     float64
}

func AppendJSONToFile(fileName string, jsonData http.Response) {
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

func getJSONAggrResults(fileName string) map[string][]http.Response {
	jsonFile, err := os.Open(fileName)
	if err != nil {
		fmt.Println("[httpagg] The result file named " + fileName + " does not exist")
		var responsesMap = make(map[string][]http.Response)
		return responsesMap
	}

	var responsesMap = make(map[string][]http.Response)
	byteValue, _ := ioutil.ReadAll(jsonFile)
	responsesCoded := json.NewDecoder(strings.NewReader(string(byteValue[:])))

	for {
		var response http.Response
		err := responsesCoded.Decode(&response)
		if err == io.EOF {
			// all done
			break
		}

		check(err)
		responsesMap[response.Request.URL] = append(responsesMap[response.Request.URL], response)

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
			AppendJSONToFile(options.FileName, response)
		}
	case "onSuccess":
		if status {
			AppendJSONToFile(options.FileName, response)
		}
	case "all":
		AppendJSONToFile(options.FileName, response)
	default:
		// by default, aggregate only invalid http responses
		if !status {
			AppendJSONToFile(options.FileName, response)
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
    <link rel="stylesheet" href="https://use.fontawesome.com/releases/v5.15.2/css/all.css" crossorigin="anonymous">
    <script src="https://code.jquery.com/jquery-3.5.1.js"></script>
    <script src="https://cdn.datatables.net/1.12.1/js/jquery.dataTables.min.js"></script>
    <style>
        .container {
            display: flex;
            /* Misc */
            width: 96%;
            height: 100%;
            margin-left: 2%;
            margin-right: 2%;
        }

        .container__left {
            /* Initially, the left takes 3/4 width */
            width: 100%;
            min-width: 30%;
            max-height: 100%;
            border: 1px solid #ece8f1;
            padding: 2%;
            overflow-y: scroll;
            font-family: Helvetica, sans-serif;
        }

        table {
            color: #3c3c64;
            font-size: 14px;
            line-height: 25px;
            border-collapse: collapse;
            width: 100%;
            max-height: 50px;
            border: 1px solid #ece8f1;
        }

        th {
            background: #f9f8fc;
            color: #5a5c87;
            font-size: 10px;
            letter-spacing: .5px;
            line-height: 18px;
            padding: 10px 20px;
            text-align: left;
            text-transform: uppercase;
            border-bottom: 1px solid #ece8f1;
            box-sizing: border-box;
            border-collapse: collapse;
            cursor: pointer;
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
            border-bottom: 1px solid rgba(125, 100, 255, 0);
            color: #7d64ff;
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

        .error {
            color: #fa3287;
        }

        .requestContainer {
            background: #3c3c64;
            margin: 0;
            padding: 15px;
            overflow-y: auto;
            text-align: left;
            transition: max-height .2s ease-in-out;
            font-family: monospace, monospace;
            font-size: 1em;
            box-sizing: border-box;
            color: #3c3c64;
            line-height: 25px;
        }

        .purple {
            color: #00cdff;
        }

        .white {
            color: white;
        }
        .failed {
            color: #ff6666 !important;
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
            color: #7d64ff;
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
            color: #7d64ff;
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
            color: #3c3c64;
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
            color: #7d64ff;
        }

        .invisible_req {
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
    </style>
</head>

<body>
    <div class="container">
        <div class="container__left">
            <table id="example">
                <thead>
                    <tr>
                        <th rowspan="2">URL</th>
                        <th colspan="3"># REQUEST</th>
                        <th colspan="4">DURATION (ms)</th>
                    </tr>
                    <tr>
                        <th>Total</th>
                        <th>Failed</th>
                        <th>HTTP >500</th>
                        <th>Min</th>
                        <th>Average</th>
                        <th>P(99.99)</th>
                        <th>Max</th>
                    </tr>
                </thead>
                <tbody>
                    {{ range $key, $value := . }}
                        <tr>
                            <td><i class="fa-solid fa-caret-down"></i></td>
                            <td>{{$key}}</td>
                            <td>{{len $value}}</td>
                            {{ $var := processHttpDuration $value }}
                            {{ $resp := $value }}
                                <td>{{$var.FailedRequest}}</td>
                                <td>{{$var.ServerError}}</td>
                                <td>{{$var.MinDuration}}</td>
                                <td>{{$var.AverageDuration}}</td>
                                <td>{{$var.P99Duration}}</td>
                                <td>{{$var.MaxDuration}}</td>
                            </tr>
                            <tr class="childTableRow">
                                <td colspan="8">
                                    <table class="table">
                                        <thead>
                                        <tr>
                                            <th>Response timestamp</th>
                                            <th>Status</th>
                                            <th>Method</th>
                                            <th>Duration (ms)</th>
                                        </tr>
                                        </thead>
                                        {{ range $resp }}
                                            <tr>
                                                <td><a>{{.Headers.Date}}</a></td>

                                                {{ if ge (.Status) 500 }}
                                                    <td class="failed">{{.Status}}</td>
                                                {{ else }}
                                                    <td>{{.Status}}</td>
                                                {{ end }}

                                                <td>{{.Request.Method}}</td>

                                                {{ if gt (.Timings.Duration) 1000.00 }}
                                                    <td class="failed">{{.Timings.Duration}}</td>
                                                {{ else }}
                                                    <td>{{.Timings.Duration}}</td>
                                                {{ end }}
                                            </tr>
                                        {{ end }}
                                    </table>
                                </td>
                            </tr>
                        {{ end }}
                </tbody>
            </table>
        </div>
    </div>

    <script type="module">
        $(document).ready(function () {
            $('#example').DataTable({
                "language": {
                    "lengthMenu": '_MENU_',
                    "search": '<i class="search"></i>',
                    "searchPlaceholder": "Search",

                },
                order: []
            });

            // change testform
            document.querySelectorAll("textarea").forEach(element => {
                function autoResize(el) {
                    el.style.height = el.scrollHeight + 'px';
                }
                autoResize(element);
                element.addEventListener('input', () => autoResize(element));    
            });

            $(document).on("click", 'table tr', function() {
                $('table tr').css('background','#ffffff');
                $(this).css('background','#f9f8fc');

                var data = $('table').DataTable().cells( selectedRow, '' ).render( 'display' );
                var selectedRow = data.row(this).index();

                $('.invisible_req').css('display','none');
                $('.invisible_req').eq(selectedRow).css('display','block');
            });

            $(document).on("click", 'thead tr', function() {
                $('table tr').eq(1).trigger('click');
            });

            $(document).on("click", '.paginate_button', function() {
                $('table tr').eq(1).trigger('click');
            });

            $('table tr').eq(1).trigger('click');
        });
    </script>
</body>

</html>
	`

	// temp := template.Must(template.New("index.txt").Funcs(funcMap).ParseFiles("index.txt"))
	var responsesMap = getJSONAggrResults(httpaggResultsFileName)
	temp, err := template.New("index.txt").Funcs(template.FuncMap{
		"processHttpDuration": func(arrResponse []http.Response) HttpObject {
			var err500, fail int
			var tmp = make([]float64, 0, len(arrResponse))
			for _, element := range arrResponse {
				if element.Status >= 500 {
					err500 += 1
				} else if (element.Status == 0) || (element.Status >= 400 && element.Status < 500) {
					fail += 1
				}
				tmp = append(tmp, element.Timings.Duration)
			}
			min, _ := stats.Min(tmp)
			avg, _ := stats.Mean(tmp)
			p99, _ := stats.Percentile(tmp, 99.99)
			max, _ := stats.Max(tmp)
			return HttpObject{
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
