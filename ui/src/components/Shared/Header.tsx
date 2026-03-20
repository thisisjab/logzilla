import { GiScorpion } from "react-icons/gi"

function Header() {
    return (
        <header className="p-5 flex flex-col items-start gap-1">
            <div className="flex gap-2 items-center">
                <h1 className="text-white font-extrabold text-2xl font-mono">Logzilla</h1>
                <GiScorpion size={18} className="text-red-500" />
            </div>
            <p className="text-gray-300 text-xs sm:text-sm">Rapid log aggregation solution that works with no hustle.</p>
        </header>
    )
}

export default Header
