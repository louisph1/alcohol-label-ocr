For an explaination on what this is, read instructions.md


#6 Note:
If this was an actual problem, I believe the solution would not be to make a new prototype/program (as the systems administrator said, that's years away), but to just upgrade the old system to process the OCR on the backend with another thread and store it (like this program does). Instead of wasting time doing it on the front end when an employee starts to process a file, just do it after an upload automatically. You could keep the algorithm with the 40 second delay that way, but upgrading should not be too difficult either. You could roll something like that out overnight without system/ui changes or regulatory hurdles.

#6 Deployment:
The server opens two ports for two access points. One for users/companies and one for employees. Ideally you would buy a domain name to make both easier to find, then use a firewall/ or reverse proxy to protect the employee site from public internet access. A normal login is fine too if you want to allow work from home.

#6 Scope:
This is not scalable because it is a prototype and only intended to demonstrate addition of ML tools. On a real program there should be protections to prevent multiple users working on the same case, like seperate employee queues or claiming of batches. There is no protection against DOS attacks/spam, which would be very bad on a real system.

#6 Dependencies
This program is entirely self hosted with open source software so contractor payments, external connections, and downtime for dependencies are non-issues. It uses SQLite for the database and tesseract for the optical character recognition. I used go because it's a fast language that's simple to use and designed to make web backends.

#6 AI use
I used AI to generate an initial frontend/backend and then fixed the database/inputs to how I wanted and wrote the OCR logic. I could have used it more to get the forms right, but live and learn.


#6 Other note
The forks, stargazers, watchers, repo, etc are all public so anyone can see other people's submissions and the personal account of the guy that made this.