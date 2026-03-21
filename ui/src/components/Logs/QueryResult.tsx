import type { LogRecord } from "../../api/models/LogRecord"
import { formatDate } from "../../utils/formatDate"
import LogLevelLabel from "./LogLevelLabel"

interface Props {
    logs: LogRecord[]
}

function QueryResult({ logs }: Props) {
    return (
        <div className=" overflow-x-auto min-w-full max-h-full rounded">
            <table className=" bg-gray-900 p-3 rounded min-w-full table-auto border-separate border-spacing-2 overflow-auto">
                <thead className="">
                    <tr>
                        <th className="w-40 px-3 py-2 text-left text-xs font-bold text-gray-300 uppercase tracking-wider">ID</th>
                        <th className="w-30 px-3 py-2 text-left text-xs font-bold text-gray-300 uppercase tracking-wider">Timestamp</th>
                        <th className="w-40 px-3 py-2 text-left text-xs font-bold text-gray-300 uppercase tracking-wider">Source</th>
                        <th className="w-20 px-3 py-2 text-left text-xs font-bold text-gray-300 uppercase tracking-wider">Level</th>
                        <th className="min-w-100 px-3 py-2 text-left text-xs font-bold text-gray-300 uppercase tracking-wider">Message</th>
                        <th className="min-w-100 px-3 py-2 text-left text-xs font-bold text-gray-300 uppercase tracking-wider">Metadata</th>
                    </tr>
                </thead>
                <tbody className="align-top font-mono">
                    {logs.map(log => {
                        return <tr className="border border-gray-500" key={log.id}>
                            <td className="px-3 py-2 text-left text-wrap text-xs font-medium text-gray-400">{log.id}</td>
                            <td className="px-3 py-2 text-left whitespace-break-spaces text-xs text-gray-400">{formatDate(log.timestamp)}</td>
                            <td className="px-3 py-2 text-left whitespace-nowrap text-sm text-gray-300">{log.source}</td>
                            <td className="px-3 py-2 text-left whitespace-nowrap"><LogLevelLabel level={log.level} /></td>
                            <td className="px-3 py-2 text-sm text-gray-300 text-wrap">{log.message}</td>
                            <td>
                                <textarea
                                    className="w-full max-h-75 rounded p-1 bg-gray-800 text-sm text-gray-300 border border-gray-700 resize-y outline-none font-mono"
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
