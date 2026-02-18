# TODOs

- [ ] Support for saving unprocessed logs (due to processor failure)
- [ ] Add job for re-processing failed logs

## Documentation

- [x] Add more sample configuration files and explain all possible values

## Sources

- [ ] Add rotating file source
- [ ] Add sources that can be used within SDKs

## Processors

- [ ] Add fastjson processor
- [ ] Add regex processor

## Engine

- [x] Remove `IsProcessed ` field since we don't need it anymore
- [x] Handle panics using a recovery
- [x] Check if configuration works as expected
- [x] Check if sources are saved correctly

## Lua Processor

- [x] Check if log source is saved correctly
- [x] Make sure JSON support works as expected
- [ ] Test execution speed + ram usage
- [x] Check if utilities included with lua VM is enough by checking datetime parsing, regex, map, etc
- [x] Test if lua maps are parsed decently
- [x] Validate VM is sandboxed (even at beginner level)

## UI

- [ ] Add basic UI functionality

## Querier

- [ ] Research about options
