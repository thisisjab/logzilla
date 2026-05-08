import Footer from "../components/Shared/Footer"
import { useHealthcheck } from "../hooks/useHealthcheck"

function HealthcheckPage() {
    const { data, isLoading, isError } = useHealthcheck()




    return (
        <>
            <main className="max-w-full text-nowrap m-5 grow flex flex-col justify-center align-center text-center text-gray-500">
                {isError && <p >Unexpected error. Please check console for more info.</p>}
                {isLoading && <p >Waiting for response...</p>}
                {data?.data &&
                    <div className="flex justify-center">
                        <textarea
                            className="w-100 min-h-30 rounded p-1 bg-gray-200 dark:bg-gray-800 text-xs text-gray-500 dark:text-gray-300 border border-gray-300 dark:border-gray-700 resize-y outline-none font-mono overflow-hidden"
                            readOnly
                            defaultValue={JSON.stringify(data?.data, null, 4)}
                        ></textarea>
                    </div>
                }
            </main >
            <Footer />
        </>
    )
}

export default HealthcheckPage
