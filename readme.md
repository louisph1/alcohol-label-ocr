For an explaination on what this is, read instructions.md

## Setup and run
Install tesserect (os dependent, should be a package on most linux distributions)

Then run:

`go run .`

Or build executable with

`go build .`

### Note:
If this was an actual problem, I believe the solution would not be to make a new prototype/program (as the systems administrator said, that's years away), but to just upgrade the old system to process the OCR on the backend with another thread and store it (like this program does). Instead of wasting time doing it on the front end when an employee starts to process a file, just do it after an upload automatically. You could keep the algorithm with the 40 second delay that way, but upgrading should not be too difficult either. You could roll something like that out overnight without system/ui changes or regulatory hurdles.

### Deployment:
The server opens two ports for two access points. One for users/companies and one for employees. Ideally you would buy a domain name to make both easier to find, then use a firewall or reverse proxy to protect the employee site from public internet access if an employee login is insufficient.

Since Azure is used, a D-family VPS is probably sufficient (N-series if switching to a multimodal LLM, see below). 150,000 applications is just 17/hr and normal web server stuff like storage and bandwidth will be the main bottlenecks most likely. Self hosting the whole thing would work too.

There are some scalability limits in this prototype that could be fixed in a real release without much issue. (see scope section.)

### Dependencies
This program is entirely self hosted with open source software so contractor payments, external connections, and downtime for dependencies are non-issues. It uses SQLite for the database and Tesseract for the optical character recognition. I used go because it's a fast language that's simple to use and designed to make web backends.

### Used OCR Model
Tesseract is used because it is lightweight, CPU based, self-hosted, and open-source. The image is pre-processed for clarity and it uses a non-LLM AI and tries to roughly match the inputs to the image.

### Room for improvement
A multimodal LLM would be better because it could determine things like text font/size requirements, as well as determining legal compliance and different rules for types natively. Rules could be changed by non-technical employees just editing text files. GPT-4o or Claude would have been great, but the requirement for external APIs is bad, as well as the constant model changes that occur.

An open-weight model like Google Gemma 4 would be perfect, but I don't own a powerful GPU to test this. I tried deepseek and it worked well for a TTB label, but the government probably doesn't want to use Chinese AI, even if it's open-weight.

### Scope:
This is not scalable because it is a prototype and only intended to demonstrate addition of ML tools. On a real program there should be protections to prevent multiple users working on the same case, like seperate employee queues or claiming of batches. There is no protection against DOS attacks/spam.

For a real system it's likely better to put the web user interface on a different server and use MySQL instead of SQLite.

### Other note
The forks, stargazers, watchers, repo, etc are all public so anyone can see other people's submissions and the personal account of the guy that made this.