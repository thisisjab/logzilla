import LogLevelLabel from "./LogLevelLabel"

function QueryResult() {
    return (
        <div className=" overflow-x-auto min-w-full max-h-full rounded">
            <table className=" bg-gray-700 p-3 rounded min-w-full table-auto border-separate border-spacing-2 overflow-auto">
                <thead className="">
                    <tr>
                        <th className="w-30 px-3 py-2 text-left text-xs font-bold text-gray-300 uppercase tracking-wider">ID</th>
                        <th className="w-30 px-3 py-2 text-left text-xs font-bold text-gray-300 uppercase tracking-wider">Timestamp</th>
                        <th className="w-40 px-3 py-2 text-left text-xs font-bold text-gray-300 uppercase tracking-wider">Source</th>
                        <th className="w-20 px-3 py-2 text-left text-xs font-bold text-gray-300 uppercase tracking-wider">Level</th>
                        <th className="min-w-100 px-3 py-2 text-left text-xs font-bold text-gray-300 uppercase tracking-wider">Message</th>
                        <th className="min-w-100 px-3 py-2 text-left text-xs font-bold text-gray-300 uppercase tracking-wider">Metadata</th>
                    </tr>
                </thead>
                <tbody className="align-top font-mono">
                    <tr className="border border-gray-500">
                        <td className="px-3 py-2 text-left whitespace-nowrap text-sm font-medium text-white">1</td>
                        <td className="px-3 py-2 text-left whitespace-nowrap text-sm text-gray-300">2012-12-13T10:10:10</td>
                        <td className="px-3 py-2 text-left whitespace-nowrap text-sm text-gray-300">main-server</td>
                        <td className="px-3 py-2 text-left whitespace-nowrap"><LogLevelLabel level={0} /></td>
                        <td className="px-3 py-2 text-sm text-gray-300 text-wrap">Hello Lorem ipsum dolor sit amet consectetur adipisicing elit. Nihil deleniti veniam quis numquam quas sunt fugiat id necessitatibus non dignissimos vel ipsum deserunt, asperiores molestias eos quisquam nulla? Eius, repellat!</td>
                        <td>
                            <textarea
                                className="w-full max-h-75 rounded p-1 bg-gray-800 text-sm text-gray-300 border border-gray-700 resize-y outline-none font-mono"
                                readOnly
                                defaultValue="{'{name: \'ali\'}'}"
                            ></textarea>
                        </td>
                    </tr>
                    <tr className="border border-gray-500">
                        <td className="px-3 py-2 text-left whitespace-nowrap text-sm font-medium text-white">1</td>
                        <td className="px-3 py-2 text-left whitespace-nowrap text-sm text-gray-300">2012-12-13T10:10:10</td>
                        <td className="px-3 py-2 text-left whitespace-nowrap text-sm text-gray-300">main-server</td>
                        <td className="px-3 py-2 text-left whitespace-nowrap"><LogLevelLabel level={1} /></td>
                        <td className="px-3 py-2 text-sm text-gray-300 text-wrap">Hello Lorem ipsum dolor sit amet consectetur adipisicing elit. Nihil deleniti veniam quis numquam quas sunt fugiat id necessitatibus non dignissimos vel ipsum deserunt, asperiores molestias eos quisquam nulla? Eius, repellat!</td>
                        <td>
                            <textarea
                                className="w-full max-h-75 rounded p-1 bg-gray-800 text-sm text-gray-300 border border-gray-700 resize-y outline-none font-mono"
                                readOnly
                                defaultValue="{'{name: \'ali\'}'}"
                            ></textarea>
                        </td>
                    </tr>
                    <tr className="border border-gray-500">
                        <td className="px-3 py-2 text-left whitespace-nowrap text-sm font-medium text-white">1</td>
                        <td className="px-3 py-2 text-left whitespace-nowrap text-sm text-gray-300">2012-12-13T10:10:10</td>
                        <td className="px-3 py-2 text-left whitespace-nowrap text-sm text-gray-300">main-server</td>
                        <td className="px-3 py-2 text-left whitespace-nowrap"><LogLevelLabel level={2} /></td>
                        <td className="px-3 py-2 text-sm text-gray-300 text-wrap">Hello Lorem ipsum dolor sit amet consectetur adipisicing elit. Nihil deleniti veniam quis numquam quas sunt fugiat id necessitatibus non dignissimos vel ipsum deserunt, asperiores molestias eos quisquam nulla? Eius, repellat!</td>
                        <td>
                            <textarea
                                className="w-full max-h-75 rounded p-1 bg-gray-800 text-sm text-gray-300 border border-gray-700 resize-y outline-none font-mono"
                                readOnly
                                defaultValue="{'{name: \'ali\'}'}"
                            ></textarea>
                        </td>
                    </tr>
                    <tr className="border border-gray-500">
                        <td className="px-3 py-2 text-left whitespace-nowrap text-sm font-medium text-white">1</td>
                        <td className="px-3 py-2 text-left whitespace-nowrap text-sm text-gray-300">2012-12-13T10:10:10</td>
                        <td className="px-3 py-2 text-left whitespace-nowrap text-sm text-gray-300">main-server</td>
                        <td className="px-3 py-2 text-left whitespace-nowrap"><LogLevelLabel level={3} /></td>
                        <td className="px-3 py-2 text-sm text-gray-300 text-wrap">Hello Lorem ipsum dolor sit amet consectetur adipisicing elit. Nihil deleniti veniam quis numquam quas sunt fugiat id necessitatibus non dignissimos vel ipsum deserunt, asperiores molestias eos quisquam nulla? Eius, repellat!</td>
                        <td>
                            <textarea
                                className="w-full max-h-75 rounded p-1 bg-gray-800 text-sm text-gray-300 border border-gray-700 resize-y outline-none font-mono"
                                readOnly
                                defaultValue="{'{name: \'ali\'}'}"
                            ></textarea>
                        </td>
                    </tr>
                    <tr className="border border-gray-500">
                        <td className="px-3 py-2 text-left whitespace-nowrap text-sm font-medium text-white">1</td>
                        <td className="px-3 py-2 text-left whitespace-nowrap text-sm text-gray-300">2012-12-13T10:10:10</td>
                        <td className="px-3 py-2 text-left whitespace-nowrap text-sm text-gray-300">main-server</td>
                        <td className="px-3 py-2 text-left whitespace-nowrap"><LogLevelLabel level={4} /></td>
                        <td className="px-3 py-2 text-sm text-gray-300 text-wrap">Hello Lorem ipsum dolor sit amet consectetur adipisicing elit. Nihil deleniti veniam quis numquam quas sunt fugiat id necessitatibus non dignissimos vel ipsum deserunt, asperiores molestias eos quisquam nulla? Eius, repellat!</td>
                        <td>
                            <textarea
                                className="w-full max-h-75 rounded p-1 bg-gray-800 text-sm text-gray-300 border border-gray-700 resize-y outline-none font-mono"
                                readOnly
                                defaultValue="{'{name: \'ali\'}'}"
                            ></textarea>
                        </td>
                    </tr>
                    <tr className="border border-gray-500">
                        <td className="px-3 py-2 text-left whitespace-nowrap text-sm font-medium text-white">1</td>
                        <td className="px-3 py-2 text-left whitespace-nowrap text-sm text-gray-300">2012-12-13T10:10:10</td>
                        <td className="px-3 py-2 text-left whitespace-nowrap text-sm text-gray-300">main-server</td>
                        <td className="px-3 py-2 text-left whitespace-nowrap"><LogLevelLabel level={5} /></td>
                        <td className="px-3 py-2 text-sm text-gray-300 text-wrap">Hello Lorem ipsum dolor sit amet consectetur adipisicing elit. Nihil deleniti veniam quis numquam quas sunt fugiat id necessitatibus non dignissimos vel ipsum deserunt, asperiores molestias eos quisquam nulla? Eius, repellat!</td>
                        <td>
                            <textarea
                                className="w-full max-h-75 rounded p-1 bg-gray-800 text-sm text-gray-300 border border-gray-700 resize-y outline-none font-mono"
                                readOnly
                                defaultValue="{'{name: \'ali\'}'}"
                            ></textarea>
                        </td>
                    </tr>
                    <tr className="border border-gray-500">
                        <td className="px-3 py-2 text-left whitespace-nowrap text-sm font-medium text-white">1</td>
                        <td className="px-3 py-2 text-left whitespace-nowrap text-sm text-gray-300">2012-12-13T10:10:10</td>
                        <td className="px-3 py-2 text-left whitespace-nowrap text-sm text-gray-300">main-server</td>
                        <td className="px-3 py-2 text-left whitespace-nowrap text-sm text-gray-300">INFO</td>
                        <td className="px-3 py-2 text-sm text-gray-300">Hello</td>
                        <td>
                            <textarea
                                className="w-full max-h-75 rounded p-1 bg-gray-800 text-sm text-gray-300 border border-gray-700 resize-y outline-none font-mono"
                                readOnly
                                defaultValue="{'{name: \'ali\'}'}"
                            ></textarea>
                        </td>
                    </tr>
                </tbody>
            </table>
        </div>
    )
}

export default QueryResult
