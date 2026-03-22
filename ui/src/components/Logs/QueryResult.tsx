import type { LogRecord } from "../../api/models/LogRecord"
import { formatDate } from "../../utils/formatDate"
import LogLevelLabel from "./LogLevelLabel"

interface Props {
    logs: LogRecord[]
}

function QueryResult({ logs }: Props) {
    return (
        <div className=" overflow-x-auto min-w-full max-h-full rounded">
            <table className="bg-gray-100 dark:bg-gray-900 p-3 rounded min-w-full table-auto border-separate border-spacing-2 overflow-auto">
                <thead className="">
                    <tr className="text-gray-700 dark:text-gray-300 uppercase  font-bold text-xs text-left">
                        <th className="w-40 px-3 py-2 tracking-wider">ID</th>
                        <th className="w-30 px-3 py-2 tracking-wider">Timestamp</th>
                        <th className="w-40 px-3 py-2 tracking-wider">Source</th>
                        <th className="w-20 px-3 py-2 tracking-wider">Level</th>
                        <th className="min-w-100 px-3 py-2 tracking-wider">Message</th>
                        <th className="min-w-100 px-3 py-2 tracking-wider">Metadata</th>
                    </tr>
                </thead>
                <tbody className="align-top font-mono">
                    {logs.map(log => {
                        return <tr className="text-left text-gray-500 dark:text-gray-300" key={log.id}>
                            <td className="px-3 py-2 text-wrap text-xs font-medium opacity-70">{log.id}</td>
                            <td className="px-3 py-2 whitespace-break-spaces text-xs opacity-70">{formatDate(log.timestamp)}</td>
                            <td className="px-3 py-2 whitespace-nowrap text-sm">{log.source}</td>
                            <td className="px-3 py-2 whitespace-nowrap"><LogLevelLabel level={log.level} /></td>
                            <td className="px-3 py-2 text-sm text-wrap">{log.message}</td>
                            <td>
                                <textarea
                                    className="w-full rounded p-1 bg-gray-200 dark:bg-gray-800 text-xs text-gray-500 dark:text-gray-300 border border-gray-300 dark:border-gray-700 resize-y outline-none font-mono overflow-hidden"
                                    readOnly
                                    defaultValue={JSON.stringify(log.metadata, null, 4)}
                                ></textarea>
                            </td>
                        </tr>
                    })}
                </tbody>
            </table>
        </div>
    )
}

export default QueryResult
