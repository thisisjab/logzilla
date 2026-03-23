import { GiScorpion } from "react-icons/gi"
import { Link } from "react-router-dom"

function Header() {
    return (
        <header className="p-5 flex flex-col items-start gap-1 border-b border-red-500">
            <div className="flex gap-2 items-center">
                <h1 className="font-extrabold text-2xl font-mono">Logzilla</h1>
                <GiScorpion size={18} className="text-red-500" />
            </div>
            <div className="flex flex-col sm:flex-row gap-2 justify-between text-gray-400 dark:text-gray-300 text-xs sm:text-sm w-full">
                <p className="">Rapid log aggregation solution that works with no hustle.</p>
                <Link className="underline self-end text-xs" to={"/healthcheck"}>Healthcheck</Link>
            </div>
        </header>
    )
}

export default Header
