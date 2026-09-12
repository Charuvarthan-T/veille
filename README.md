well.. veille’s worker basically calls the codeforces and codechef apis, pulls the contest data, cleans and maps it into the format i gave (see thru the codebase), then stores it in neon postgres. every run it checks which contests are now live and creates a notification record, then the notification worker picks it up and sends the email through resend. 

also, github actions runs this whole flow automatically every hour.
