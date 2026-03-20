import { useState } from "react";
import QueryInput from "./QueryInput"
import QueryResult from "./QueryResult"
import { useSearchLogs } from "../../hooks/useSearchLogs";
import LoadingIndicator from "../Shared/LoadingIndicator";

function QuerySection() {
    const [query, setQuery] = useState<string>("");
    const { data, isLoading } = useSearchLogs(query);

    // TODO: add error handling

    return (
        <main className="max-w-full text-nowrap m-5 grow flex flex-col overflow-hidden gap-3">
            <QueryInput onQuerySubmit={q => setQuery(q)} />
            {isLoading &&
                <div className="flex justify-center items-center grow ">
                    <LoadingIndicator />
                </div>}
            {data?.data.data !== undefined && <QueryResult logs={data?.data.data} />}
        </main >
    )
}

export default QuerySection
