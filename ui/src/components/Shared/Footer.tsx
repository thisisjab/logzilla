import { FaHeart } from 'react-icons/fa'

function Footer() {
    return <footer className="p-3 flex justify-center items-center gap-1">
        <p className="text-gray-400 dark:text-gray-500 text-xs font-light">Logzilla is built 100% by</p><FaHeart size={8} className="text-red-500 dark:text-red-900" />
    </footer>
}

export default Footer
