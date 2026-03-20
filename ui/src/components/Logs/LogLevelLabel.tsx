type LogLevel = 0 | 1 | 2 | 3 | 4 | 5;

interface Props {
    level: LogLevel
}


function LogLevelLabel({ level }: Props) {
    switch (level) {
        case 0: return <p className={`text-center text-sm text-yellow-500 bg-yellow-900 px-1 py-0.5 rounded uppercase`}>Unknown</p>
        case 1: return <p className={`text-center text-sm text-gray-600 bg-gray-300 px-1 py-0.5 rounded uppercase`}>Debug</p>
        case 2: return <p className={`text-center text-sm text-blue-600 bg-blue-300 px-1 py-0.5 rounded uppercase`}>Info</p>
        case 3: return <p className={`text-center text-sm text-orange-900 bg-orange-300 px-1 py-0.5 rounded uppercase`}>Warning</p>
        case 4: return <p className={`text-center text-sm text-red-700 bg-red-300 px-1 py-0.5 rounded uppercase`}>Error</p>
        case 5: return <p className={`text-center text-sm text-yellow-500 bg-red-600 px-1 py-0.5 rounded uppercase font-bold`}>Fatal</p>
    }

}

export default LogLevelLabel
