For an explaination on what this is, read instructions.md

## Setup and run
Generate an API key and put it in `apikey.txt`

Then run:

`go run .`

Or build executable with

`go build .`

# Testing
This is hosted using Hetzner

Customer portal: `5.161.124.101:8080` \
Employee portal: `5.161.124.101:8081`

### Notes:
There is a large delay (10-40s) between uploading and getting the AI results for an image, HOWEVER, this is designed to happen after upload and cache the results before a TTB employee even sees the label. Therefore it should be seamless for actual employees, but there is a visible delay for testers.

Although a public API may be an issue, I used an open weight model (Gemma 4 31B) so it could be self hosted if that's an issue. Google endpoints shouldn't be blocked anyways, and if you're using an Azure VPS you can probably just choose not to have a firewall, since API requests are all server side.

If this was an actual problem, I believe the solution would not be to make a new prototype/program (as the systems administrator said, that's years away), but to just upgrade the old system to process the OCR on the backend with another thread and store it (like this program does). Instead of wasting time doing it on the front end when an employee starts to process a file, just do it after an upload automatically. You could keep the algorithm with the 40 second delay that way, but upgrading should not be too difficult either. You could roll something like that out overnight without system/ui changes or regulatory hurdles.

### Deployment:
The server opens two ports for two access points. One for users/companies and one for employees. Ideally you would buy a domain name and use a reverse proxy to make both easier to find, then use a login to protect the employee site.

Since Azure is used, a D-family VPS is probably sufficient (N-series if self hosting AI). 150,000 applications is just 17/hr and normal web server stuff like storage and bandwidth will be the main bottlenecks most likely.

# Design decisions
### Used OCR Model
I used tesseract originally, but abandoned it because it got bad results, although it was fast and open source. Gemma 4 is better because it recognizes multiple fonts/text sizes, is smart enough to determine things like boldness/visibility of the government warning, different formats for ABV, etc. No need for janky regex. Theoretically non-technical employees could give it new instructions by modifying the prompt as well, but that's out of scope here. It just sends back JSON with any issues it has.

Sometime it suffers from AI schizophrenia, mostly saying the GOVERNMENT WARNING is not capitalized when it is, but it still produces a good list of points for employees to look at. Having the thinking and resolution on highest helps somewhat. I consider this minor annoyance an acceptable trade-off for the more intelligent behavior of an LLM, since it's intended as a tool to help employees save time, it's better if it's oversensitive and marks things that don't exist rather than miss things that do. The model can easily be changed to a better one later as the technology keeps improving.

### Dependencies
This program  uses SQLite for the database and Gemma 4 (through google API) for the optical character recognition. I used go because it's a fast language that's simple to use and designed to make web backends.

### Scope:
This is not scalable because it is a prototype and only intended to demonstrate addition of ML tools. On a real program there should be protections to prevent multiple users working on the same case, like seperate employee queues or claiming of batches. There is no protection against DOS attacks/spam.

For a real system it's likely better to put the web user interface on a different server (so the employees don't stop working if the frontend gets DDOSed/needs downtime) and use MySQL instead of SQLite.

### Other note
The forks, stargazers, watchers, repo, etc are all public so anyone can see other people's submissions and the personal account of the guy that made this.