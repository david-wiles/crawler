# Crawl Project

This is a very simple web crawler to be used in a data science project.
It will crawl the web and follow links, saving all information about the request and 
response for further use. The body itself is saved into a separate file for later 
analysis.

## Features 

__Request__

* URL
* Headers
* Method
* Request timestamp

__Response__

* Status Code
* Status Message
* Headers
* Response Time

__Content__

* Title
* Headings
* Body text
* Head meta tags
* json+ld
* Links
* Images
* Response size
* JavaScript/CSS Resource URL
* Navigation

__Meta__

_Must be computed after requests have completed_

* Referrers
