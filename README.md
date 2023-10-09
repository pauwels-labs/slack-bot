# slack-bot

This is a simple Slack bot that allows for any number of plugins
("handlers") to be added to it dynamically, allowing it to do any
number of totally custom actions by simply adding a bit of code that
conforms to a handler interface.

A handler can also be sourced from an external package for extra fun!

## Writing a new handler

New handlers can be added to the `pkg/handlers` module, they simply
need to conform to the `SlackSlashCommandHandler` interface defined in
the `pkg/slack/bot.go` file.

A description of each function of the interface is provided below.

```
Handle(arguments []string, request SlackSlashCommandBody) (*SlackResponse, error)
```

This is the primary meat of the interface, and is what the internal
Slack bot logic will call when a slash command is received from
Slack. A slash command looks a bit like this: `/bot-name [command]
[args...]`

The Slack bot code will call `Handle(...)` with the `[args...]`
provided in the `arguments` parameter. The `request` parameter
contains the raw request as sent by Slack in case any of that data is
necessary for handling. The `SlackSlashCommandBody` struct is also
defined in the `pkg/slack/bot.go` file.

Once the requested action has been handled, this function must return
either an error or a pointer to a `SlackResponse` struct, also defined
in `pkg/slack/bot.go`.

The `SlackResponse` type is simply the object expected by the Slack
API as a response to an interaction with the bot. It contains a
`ResponseType` field which should be set to either `ephemeral` if the
response should only be seen by the requester, or `in_channel` if it
should be seen by everyone in the channel. The `Text` field should be
the contents of that response.

Currently, this interface and workflow is simple and optimized for
receiving commands and providing a reponse in the channel. In the
future, it may be modified to support more complex workflows involving
modals or other more advanced Slack features.

```
CommandName() string
```

This should return a single word which is the expected command that a
user will pass in the Slack slash command to invoke this handler. As
an example, for the `EchoHandler` defined in `pkg/handlers/echo.go`,
the `CommandName()` function returns `echo`, which means it will be
called when a user types in `/bot-name echo ...`.

```
CommandArguments() string
```

This should return the list of arguments this command expects to
receive. It is used exclusively for when the bot generates the help
text available at `/bot-name help`.

```
CommandDescription() string
```

This should return a short text description of this handler, what it
does, and how to use it. It is used exclusively for when the bot
generates the help text available at `/bot-name help`.

## Adding a new handler to the bot

Once you've written a new handler, it needs to be added to the
initialization of the Slack bot. This is very simple, just go to
`cmd/slack-bot/main.go` and find the `CreateHandlers()` function.  You
can call the constructors of your various handlers and add them to the
array returned here.

## Help text and other conveniences

Once the handler is written and added to `CreateHandlers()`, no
further action is required. Simply build and push your bot, the
internal bot logic will automatically handle the new command word and
will add a help text for your command to the `/bot-name help` command.

## Creating a Slack bot and connecting it to this code

1. Create a new Slack bot by logging in to your workpace on the Slack
   website and then going to the New App page or clicking
   [here](https://api.slack.com/apps?new_app=1).
   
2. Go to the OAuth &amp; Permissions section and assign the `commands`
   OAuth scope to your app.
   
3. Go to the Slash Commands section and create a new command using
   whatever you want as the command text (typically the app name is a
   good idea), so that the general flow is `/bot-name [handler name]
   [args...]`. Set the URL to the URL that you've set up this Slack
   bot to listen under.
   
4. Wherever this Slack bot runs, it needs its secret Slack signing key
   in order to properly verify HTTP requests coming from Slack. Fetch
   this signing key by going to the Basic Information section and
   copying the Signing Secret to put it inside the `slack.signingkey`
   value in the bot's config.
   
5. Install the app to your workspace by going to Basic Information >
   Install your app. The slash command should now be available across
   all channels in your workspace.
