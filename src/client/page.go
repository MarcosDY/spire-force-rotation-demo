package main

import "html/template"

const markup = `
<!DOCTYPE html>
<html>
    <head>

    </head>
    <body>
        <style>
            body {
                font-family: Arial, Helvetica, sans-serif;
                margin: 0 20%;
            }
            .right {
                text-align: right;
            }
            .center {
                text-align: center;
            }
            .data-table {
                width: 100%;
            }
            .data-table, .data-table td, .data-table th {
                border-spacing: 0;
                border: 1px solid black;
            }
            .error {
                color: red;
            }
        </style>
        {{if .CustomersErr}}
	<div class="error">Customers service unavailable: {{.CustomersErr}}</div>
        {{end}}
        {{if .ProductsErr}}
	<div class="error">Products service unavailable: {{.ProductsErr}}</div>
        {{end}}
	<div>
	    <h1>Products</h1>
	</div>
        <table class="data-table">
            <caption class="right">Last Updated: {{.LastUpdated.Format "Jan 2 15:04:05"}}</caption>
            <thead>
                <tr>
                    <th scope="col">Name</th>
                    <th scope="col">Stock</th>
                </tr>
            </thead>
	    <tbody>
		{{range .Products}}
                    <tr>
                        <td scope="row">{{.Name}}</td>
                        <td scope="row">{{.Stock}}</td>
                    </tr>
                {{end}}
            </tbody>
        </table>
	<div>
	    <h1>Customers</h1>
	</div>
        <table class="data-table">
            <caption class="right">Last Updated: {{.LastUpdated.Format "Jan 2 15:04:05"}}</caption>
            <thead>
                <tr>
                    <th scope="col">Name</th>
                    <th scope="col">Address</th>
                </tr>
            </thead>
	    <tbody>
		{{range .Customers}}
                    <tr>
                        <td scope="row">{{.Name}}</td>
                        <td scope="row">{{.Address}}</td>
                    </tr>
                {{end}}
            </tbody>
        </table>
    </body>
</html>
`

var (
	page, _ = template.New("quotes").Parse(markup)
)
