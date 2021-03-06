module BuildDuration where

import Date exposing (Date)
import Date.Format
import Duration exposing (Duration)
import Html exposing (Html)
import Html.Attributes exposing (class, title)
import Time exposing (Time)

import Concourse.Build exposing (BuildDuration)

view : BuildDuration -> Time.Time -> Html
view duration now =
  Html.table [class "dictionary build-duration"] <|
    case (duration.startedAt, duration.finishedAt) of
      (Nothing, _) ->
        [ pendingLabel "pending" ]

      (Just startedAt, Nothing) ->
        [ labeledRelativeDate "started" now startedAt ]

      (Just startedAt, Just finishedAt) ->
        let
          durationElmIssue = -- https://github.com/elm-lang/elm-compiler/issues/1223
            Duration.between (Date.toTime startedAt) (Date.toTime finishedAt)
        in
          [ labeledRelativeDate "started" now startedAt
          , labeledRelativeDate "finished" now finishedAt
          , labeledDuration "duration" durationElmIssue
          ]

labeledRelativeDate : String -> Time -> Date -> Html
labeledRelativeDate label now date =
  let
    ago = Duration.between (Date.toTime date) now
  in
    Html.tr []
    [ Html.td [class "dict-key"] [Html.text label]
    , Html.td
        [title (Date.Format.format "%b %d %Y %I:%M:%S %p" date), class "dict-value"]
        [Html.span [] [Html.text (Duration.format ago ++ " ago")]]
    ]

labeledDuration : String -> Duration -> Html
labeledDuration label duration =
  Html.tr []
  [ Html.td [class "dict-key"] [Html.text label]
  , Html.td [class "dict-value"] [Html.span [] [Html.text (Duration.format duration)]]
  ]

pendingLabel : String -> Html
pendingLabel label =
  Html.tr []
  [ Html.td [class "dict-key"] [Html.text label]
  , Html.td [class "dict-value"] []
  ]
