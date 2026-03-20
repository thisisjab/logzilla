import Footer from "../components/Shared/Footer"

function HealthcheckPage() {
    return (
        <>
            <main className="max-w-full text-nowrap m-5 grow flex flex-col justify-center align-center">
                <p className="text-center">Server is up and running...</p>
            </main >
            <Footer />
        </>
    )
}

export default HealthcheckPage
