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
	PassedRequest   int
	ServerTimeout   int
	RequestError    int
	ServerError     int
	TotalRequest    int
	TotalFailed     int
	AverageDuration float64
	MaxDuration     float64
	P95Duration     float64
	P9999Duration   float64
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
	var trimmedURL string
	switch {
	case strings.Contains(url, "mrapp"), true:
		trimmedURL = strings.Replace(url, match, "{MRAPP}", 1)
	case strings.Contains(url, "engage"):
		trimmedURL = strings.Replace(url, match, "{ENGAGE}", 1)
	case strings.Contains(url, "online-appointment"):
		trimmedURL = strings.Replace(url, match, "{WIDGET}", 1)
	case strings.Contains(url, "v1"):
		trimmedURL = strings.Replace(url, match, "{EXTERNAL}", 1)
	}

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

func formatTooltip(serverTimeout, requestError, serverError int) string {
	return fmt.Sprintf("Timeout Error: %d\n4xx Error: %d\nServer Error: %d", serverTimeout, requestError, serverError)
}

func (*Httpagg) GenerateRaport(httpaggResultsFileName string, httpaggReportFileName string) {
	const tpl = `
	<html lang="en">

<head>
    <meta charset="utf-8" />
    <title>MR API Performance Report</title>
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <link rel="icon" href="https://medirecords.com/wp-content/uploads/2020/02/logo.svg" type="image/x-icon">
    <link rel="preconnect" href="https://fonts.googleapis.com">
    <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
    <link href="https://fonts.googleapis.com/css2?family=Plus+Jakarta+Sans:ital,wght@0,200..800;1,200..800&family=Schibsted+Grotesk:ital,wght@0,400..900;1,400..900&display=swap" rel="stylesheet">
    <link rel="stylesheet" href="https://use.fontawesome.com/releases/v5.15.2/css/all.css" crossorigin="anonymous">

    <link rel="shortcut icon" href="https://medirecords.com/wp-content/uploads/2020/02/logo.svg" type="image/png">
    <script src="https://code.jquery.com/jquery-3.5.1.js"></script>
    <script src="https://cdn.datatables.net/2.0.7/css/dataTables.dataTables.min.css"></script>
    <script src="https://cdn.datatables.net/2.0.7/js/dataTables.min.js"></script>
    <style>
        body {
            margin: 2rem;
            font-family: "Schibsted Grotesk", sans-serif;
        }

        .maintitle {
            text-align: center;
            font-weight: bold;
            margin-bottom: 40px;
        }

        table {
            border-collapse: collapse;
            max-width: 100%;
            margin-bottom: 1.2rem;
            margin-right: 1.2rem;
            box-sizing: border-box;
            font-size: 1.1rem;
            table-layout: fixed; 
            word-wrap:break-word; 
        }

        @media screen {
            table {
                width: 100%;
            }
        }

        th {
            background-color: #0f72ab;
            color: rgb(255, 255, 255);
            line-height: 24px;
            padding: 0.5rem;
            text-transform: uppercase;
            cursor: pointer;
            border: 1px solid #4999c7;
        }

        th.pass {
            background-color: #0aff99;
        }

        th.fail {
            background-color: #fa4300;
        }

        th:hover {
            background-color: #01588a;
        }

        td {
            padding: 1.5rem;
            border-left: 1px solid #e2e2e283;
            border-right: 1px solid #e2e2e283;
            border-bottom: 1px solid #e2e2e2;
        }

        tr:hover {
            background-color: #e9e9e9;
        }

        .bold-text {
            font-weight: bold;
        }

        .center {
            text-align: center;
        }

        div.dt-info {
            font-size: 0.8rem;
            font-weight: 700;
            text-align: right;
            color: #696969;
        }

        .dt-paging-button {
            color: #7e7e7e;
            background-color: transparent;
            border: none;
            font-size: 16px;
            font-weight: bold;
            border-radius: 4px;
            padding: 6px 10px;
            margin: 0 2px;
        }

        .dt-paging-button:hover:not(.disabled) {
            background-color: #e2e2e2;
            color: #0b84ca;
        }

        .dt-paging-button.current {
            background-color: #e2e2e2;
            color: #0b84ca;
        }

        .dt-paging-button.disabled {
            color: #cbcbcb;
        }

        .dt-length {
            float: right;
        }

        .dt-info {
            display: none;
        }

        input {
            border: 1px solid black;
            overflow-x: auto;
            text-align: left;
            overflow: visible;
            box-sizing: border-box;
            width: 100%;
            padding: 12px;
            padding-left: 20px;
            margin-bottom: 20px;
            margin-top: 20px;
            border-radius: 8px;
            font-size: 16px;
        }

        select {
            margin: 0px 4px;
            font-size: 16px;
            padding: 4px;
            border: none;
            border-radius: 4px;
            background-color: #e5e5e5;
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
            background-color: #eff;
            color: #008
        }

        .purple-row {
            background-color: #fef;
            color: #808
        }
        
        [data-tooltip]::before {
            /* needed - do not touch */
            content: attr(data-tooltip);
            position: absolute;
            text-align: left;
            opacity: 0;
            white-space: pre; /* Preserve whitespace and newlines */
            color: #000;

            /* customizable */
            transition: all 0.15s ease;
            padding: 10px;
            border-radius: 8px;
            box-shadow: 2px 2px 1px rgb(219, 219, 219);
        }

        [data-tooltip]:hover::before {
            /* needed - do not touch */
            opacity: 1;

            /* customizable */
            background: rgb(255, 255, 255);
            margin-top: -50px;
        }

        [data-tooltip]:not([data-tooltip-persistent])::before {
            pointer-events: none;
        }
    </style>
</head>

<body>
    <h1 class="maintitle">
        <img src="https://medirecords.com/wp-content/uploads/2020/02/logo.svg" style="vertical-align:middle" width="63" height="30" viewBox="0 0 50 45" fill="none" class="footer-module--logo--_lkxx">
        API Performance Report
    </h1>
    <table id="example">
        <thead>
            <tr>
                <th>METHOD</th>
                <th>URL</th>
                <th>REQ</th>
                <th class="pass">PASS</th>
                <th class="fail">FAIL</th>
                <th>AVG</th>
                <th>MAX</th>
                <th>P(95)</th>
                <th>P(99.99)</th>
            </tr>
        </thead>
        <tbody>
            {{ range $key, $value := . }}
                <tr>
                    <td class="center bold-text">{{$key.HttpMethod}}</td>
                    <td>{{$key.UrlPattern}}</td>
                    {{ $var := processHttpDuration $value }}
                    {{ $resp := $value }}
                        <td class="center">{{$var.TotalRequest}}</td>
                        <td class="center">{{if eq $var.PassedRequest 0}}-{{else}}{{$var.PassedRequest}}{{end}}</td>
                        <td class="center" data-tooltip="{{formatTooltip $var.ServerTimeout $var.RequestError $var.ServerError}}">
                            {{if eq $var.TotalFailed 0}}-{{else}}{{$var.TotalFailed}}{{end}}
                        </td>
                        <td class="center">{{printf "%.2f" $var.AverageDuration}}s</td>
                        <td class="center">{{printf "%.2f" $var.MaxDuration}}s</td>
                        <td class="center">{{printf "%.2f" $var.P95Duration}}s</td>
                        <td class="center">{{printf "%.2f" $var.P9999Duration}}s</td>
                    </tr>
                {{ end }}
        </tbody>
    </table>

    <script type="module">
        $(document).ready(function () {
            $('#example').DataTable({
                "language": {
                    "lengthMenu": 'Show _MENU_ of _TOTAL_ Total Request',
                    "search": '',
                    "searchPlaceholder": "Search...",
                    "emptyTable": "No data",
                    "zeroRecords": 'No records found'
                },
                autoWidth: false,
                columnDefs: [
                {
                    
                    targets: 0,
                    width: '7%',
                },
                { 
                    targets: 1,
                    width: '54%',
                },
                {
                    searchable: false,
                    targets: [2,3,4],
                    width: '5%'
                },
                {
                    searchable: false,
                    targets: [5,6,7,8],
                    width: '6%'
                }
                ]
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

	var responsesMap = getJSONAggrResults(httpaggResultsFileName)
	temp, err := template.New("index.txt").Funcs(template.FuncMap{
		"processHttpDuration": func(arrResponse []HttpResponseFiltered) HttpObjectMetrics {
			var err500, err400, errNull, pass, totalFail int
			var tmp = make([]float64, 0, len(arrResponse))
			for _, element := range arrResponse {
				if element.Status == 500 {
					err500 += 1
					totalFail += 1
				} else if element.Status >= 400 && element.Status < 500 {
					err400 += 1
					totalFail += 1
				} else if element.Status == 0 {
					errNull += 1
					totalFail += 1
				} else {
					pass++
				}
				tmp = append(tmp, element.Duration)
			}
			avg, _ := stats.Mean(tmp)
			max, _ := stats.Max(tmp)
			p95, _ := stats.Percentile(tmp, 95)
			p9999, _ := stats.Percentile(tmp, 99.99)
			return HttpObjectMetrics{
				PassedRequest:   pass,
				ServerTimeout:   errNull,
				RequestError:    err400,
				ServerError:     err500,
				TotalRequest:    len(arrResponse),
				TotalFailed:     totalFail,
				AverageDuration: float64(math.Round(avg*100) / 100000),
				MaxDuration:     float64(math.Round(max*100) / 100000),
				P95Duration:     float64(math.Round(p95*100) / 100000),
				P9999Duration:   float64(math.Round(p9999*100) / 100000),
			}
		},
		"formatTooltip": formatTooltip,
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
