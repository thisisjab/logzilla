import { useRef } from "react"
import { FaFire } from "react-icons/fa"

interface Props {
    onQuerySubmit: (query: string) => void
}

function QueryInput({ onQuerySubmit }: Props) {
    const inputRef = useRef<HTMLInputElement>(null);

    const onSubmit = () => {
        const q = inputRef.current?.value

        if (q !== undefined && q.trim().length > 0) {
            onQuerySubmit(q)
        }
    }

    return (
        <div className="flex items-center bg-gray-900 px-3 rounded">
            <span className="text-green-400">{'$'}</span>
            <input ref={inputRef} onKeyDown={e => {
                if (e.key === "Enter") onSubmit()
            }} placeholder="Query..." type="text" className="w-full px-2 py-1 my-2 outline-0 font-mono border-transparent text-white" />
            <button onClick={onSubmit} className="rounded text-red-500 hover:cursor-pointer hover:text-green-500 px-1.5 py-0.5 font-bold transition-colors"><FaFire size={15} /></button>
        </div>
    )
}

export default QueryInput
