import { FaFire } from "react-icons/fa"

function QuerySection() {
    return (
        <div className="flex items-center bg-gray-700 px-3 rounded">
            <span className="text-green-400">{'$'}</span>
            <input placeholder="Query..." type="text" className="w-full px-2 py-1 my-2 outline-0 font-mono border-transparent text-white" />
            <button className="rounded text-red-500 hover:cursor-pointer hover:text-green-500 px-1.5 py-0.5 font-bold transition-colors"><FaFire size={15} /></button>
        </div>
    )
}

export default QuerySection
