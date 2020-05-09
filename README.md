This is a program which:

  - Downloads the "from" and "to" metadata of every item in your Gmail inbox
  - Calculates a count of email by sender
  - Allows you to bulk "trash" all emails from a sender

Dependencies:
  - MongoDB
  - Personal Gmail API key: https://console.developers.google.com/apis/api/gmail.googleapis.com/credentials 

To run:
  - go run gmail-deleter/cmd/deleter

You will first want to run with the `-report` option, which will iterate
through each email in your Gmail inbox and save basic metadata. Then, you
should run with `-report`, which will tabulate the top 100 people who have
sent you the most email. Finally, you can run with `-delete-from <email>`
which will send all emails from that sender to the trash. It does not
permanently delete messages.

TODO:
  - Conform to Gmail API rate limits

Contributing:

  1. If you want to contribute any changes, please create an issue first
     so we can discuss prior to maing your PR. Of course, you are welcome
     to fork, modify, and distribute this code with yoru changes in accordance
     with the LICENSE.

  2. I don't guarantee that I will keep this repo up to date, or that I will
     respond in any sort of timely fashion! Your best bet for any change is
     to keep PRs small and focused.
