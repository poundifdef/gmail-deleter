This is a program which:

  - Downloads the "from" and "to" metadata of every item in your Gmail inbox
  - Calculates a count of email by sender
  - Allows you to bulk "trash" all emails from a sender
  - Respects the GMail API rate limits

TODO:
  - Clean up old windows. If we've passed a sliding window that is very old, then 
    remove from db.
  - Reset threads which are stuck in "fetching" or "deleting" state
  - Does not currently check for rate limit response codes from API. Should
    check for HTTP 403 or 429. https://developers.google.com/gmail/api/v1/reference/quota
  - Save position of "next page" when downloading emails, in case this is interrupted
  - Option to fetch individual threads, rather than starting the process of listing 
    all threads from scratch
    
Dependencies:
  - Personal Gmail API key: https://console.developers.google.com/apis/api/gmail.googleapis.com/credentials 

To run:
  - go run gmail-deleter/cmd/deleter

You will first want to run with the `-download` option, which will iterate
through each email in your Gmail inbox and save basic metadata. Then, you
should run with `-report`, which will tabulate the top 100 people who have
sent you the most email. Finally, you can run with `-delete-from <email>`
which will send all emails from that sender to the trash. It does not
permanently delete messages.

Contributing:

  1. If you want to contribute any changes, please create an issue first
     so we can discuss prior to making your PR. Of course, you are welcome
     to fork, modify, and distribute your changes in accordance with the
     LICENSE.

  2. I don't guarantee that I will keep this repo up to date, or that I will
     respond in any sort of timely fashion! Your best bet for any change is
     to keep PRs small and focused.
