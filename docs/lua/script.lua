function parse_log(slog)
    local json = require("json")

    -- Use json.decode to parse the string and have a valid lua table
    data = json.decode(slog)

    -- You can also do:
    -- local success, data = pcall(json.decode, slog)
    -- This way you know if the operation was successful or not.

    -- Please refer to lua documentation to know about all the operations you can perform through lua.
    m = data['message']
    m = m:lower()

    -- At the end you must return level (string), message (string), timestamp (string), metadata (table or nil)
    return data["level"],
           m,
           data["timestamp"],
           data["metadata"]
end
