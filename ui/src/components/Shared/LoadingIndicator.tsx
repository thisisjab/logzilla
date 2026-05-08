import { FaRotate } from "react-icons/fa6"
import './LoadingIndicator.css'



function LoadingIndicator() {
    return (
        <div className="flex justify-center">
            <FaRotate size={16} className="text-red-400 rotate" />
        </div>
    )
}

export default LoadingIndicator
