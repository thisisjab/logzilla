import { GiScorpion } from "react-icons/gi";
import "./App.css";
import { FaHeart } from "react-icons/fa";

function App() {
  return (
    <div className="bg-gray-800 h-lvh text-white flex flex-col">
      <header className="p-5 flex flex-col items-start gap-3">
        <div className="flex gap-2 items-center">
          <h1 className="text-white font-extrabold text-2xl font-mono">Logzilla</h1>
          <GiScorpion size={18} className="text-red-500" />
        </div>
        <p className="text-gray-300 text-sm">Rapid log aggregation solution that works with no hustle.</p>
      </header>
      <div className="bg-red-400" style={{ height: '1px' }}></div>
      <main className="max-w-full text-nowrap m-5 grow flex flex-col overflow-hidden gap-3">
        <div className="flex items-center bg-gray-700 px-3 rounded">
          <span className="text-green-400">{'$'}</span>
          <input type="text" className="w-full px-2 py-1 my-2 outline-0 font-mono border-transparent text-white" />
          <button className="rounded bg-red-500 hover:cursor-pointer hover:bg-red-800 px-1.5 py-0.5 font-bold transition-colors">Query</button>
        </div>
        <div className=" overflow-x-auto min-w-full max-h-full">
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
            <tbody className="align-top">
              <tr className="border border-gray-500">
                <td className="px-3 py-2 text-left whitespace-nowrap text-sm font-medium text-white">1</td>
                <td className="px-3 py-2 text-left whitespace-nowrap text-sm text-gray-300">2012-12-13T10:10:10</td>
                <td className="px-3 py-2 text-left whitespace-nowrap text-sm text-gray-300">main-server</td>
                <td className="px-3 py-2 text-left whitespace-nowrap text-sm text-gray-300">INFO</td>
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
      </main >
      <footer className="p-1 flex justify-center items-center">
        <p className="text-gray-600 text-xs mr-2">Logzilla is built 100% by</p><FaHeart size={8} className="text-red-900"/>
      </footer>
    </div >
  );
}

export default App;
