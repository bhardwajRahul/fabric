# Package `coreservices/smtpingress`

The SMTP ingress microservice listens on port `:25` for incoming email messages. An app can listen to the appropriate event in order to process and act upon the email message.

Use the following prompt to listen to the event:

> HEY CLAUDE...
>
> Add an inbound event sink to the OnIncomingEmail event of the SMTP ingress service.
