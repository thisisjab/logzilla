import { useState } from "react";
import QueryInput from "./QueryInput"
import QueryResult from "./QueryResult"
import { useSearchLogs } from "../../hooks/useSearchLogs";
import LoadingIndicator from "../Shared/LoadingIndicator";
import { AxiosError } from "axios";

function QuerySection() {
    const [query, setQuery] = useState<string>("");
    const { data, isLoading, error, isError } = useSearchLogs({query: query});

    // TODO: add error handling

    return (
        <main className="max-w-full text-nowrap m-5 grow flex flex-col overflow-hidden gap-3">
            {/* Query input for writing queriers */}
            <QueryInput onQuerySubmit={q => setQuery(q)} />

            {/* Dirty error handling

            DO NOT PANIC!

            This is really intentional since I was not in mood of adding react-hook-form, and other boilerplates.
            Plus, it works just fine.
            If we introduced other forms later, we can switch to react-hook-form + zod
            */}
            {isError && error instanceof AxiosError && error.status === 422 && <div>Query error: {error.response?.data?.metadata.fields.query}</div>}
            {isError && error instanceof AxiosError && error.status === 400 && <div>Query error: {error.response?.data?.message}</div>}
            {isError && !(error instanceof AxiosError ) &&  <div>Unexpected error.</div>}


            {/* Loading indicator */}
            {isLoading &&
                <div className="flex justify-center items-center grow ">
                    <LoadingIndicator />
                </div>}

            {/* Table for showing result */}
            {data?.data.data !== undefined && <QueryResult logs={data?.data.data} />}
        </main >
    )
}

export default QuerySection
