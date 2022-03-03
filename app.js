// Dependencies
const puppeteer = require('puppeteer');
const { writeFileSync, readFileSync, promises: { access } } = require('fs');
const { parse } = require('json2csv');

// Global vars for csv parser
const fields = ['title', 'content'];
const opts = { fields };

// Command line args
const myArgs = process.argv.slice(2);

// Check if the url is missing
if (!myArgs[0]) {
    console.log('Missing URL')
    process.exit(1);
}

/**
 * Check if the given file exists
 * @param {String} filePath 
 * @returns {Promise<Boolean>}
 */
const fileExists = async (filePath) => {
    try {
        await access(filePath)
        return true
    } catch {
        return false
    }
}

/**
 * Scrape the page
 * @param {Array<String>} urlList 
 * @returns {Promise<Undefined | Error>}
 */
const scrap = async (urlList) => {
    try {

        // Launch the browser
        const browser = await puppeteer.launch({
            headless: false,
            devtools: false,
            defaultViewport: {
                width: 1920,
                height: 1080,
            },
        });

        // Open a new page
        const page = await browser.newPage();

        const cookiesAvailable = await fileExists('./cookies.json');

        if (!cookiesAvailable) {

            // Navigate to the page below
            await page.goto(myArgs[0]);

            // Log the cookies
            const cookies = await page.cookies();
            const cookieJson = JSON.stringify(cookies);
            writeFileSync('cookies.json', cookieJson);

            // Close the browser
            return await browser.close();
        }

        // Set Cookies
        const cookies = readFileSync('cookies.json', 'utf8');
        const deserializedCookies = JSON.parse(cookies);
        await page.setCookie(...deserializedCookies);

        // Navigate to the page below
        await page.goto(myArgs[0]);

        await page.waitForTimeout(1000);

        // Add 1st review page to the urlList
        urlList.unshift(myArgs[0]);


        const reviewInfo = []

        for (let index = 0; index < urlList.length; index++) {
            // Navigate to the page below
            await page.goto(urlList[index]);
            await page.waitForTimeout(1000);

            // Determin current URL
            const currentURL = page.url();

            console.log(`Scraping: ${currentURL} | ${urlList.length - index} Pages Left`);

            // In browser code
            // Extract comments title
            const commentTitle = await page.evaluate(async () => {

                // Extract a tags
                const commentTitleBlocks = document.getElementsByClassName('fCitC')

                // Array to store the comment titles
                const titles = [];

                // Higher order functions don't work in the browser
                for (let index = 0; index < commentTitleBlocks.length; index++) {
                    titles.push(commentTitleBlocks[index].children[0].innerText);
                }

                return titles;
            });

            // Extract comments text
            const commentContent = await page.evaluate(async () => {

                const commentContentBlocks = document.getElementsByTagName('q')

                // Array use to store the comments
                const comments = []

                for (let index = 0; index < commentContentBlocks.length; index++) {
                    comments.push(commentContentBlocks[index].children[0].innerText)
                }

                return comments
            })

            reviewInfo.push({ titles: commentTitle, content: commentContent })


        }


        return reviewInfo

    } catch (err) {
        throw err;
    }
};

/**
 * Extract review page url
 * @returns {Promise<Undefined | Error>}
 */
const extractAllReviewPageUrls = async () => {
    try {

        // Launch the browser
        const browser = await puppeteer.launch({
            headless: false,
            devtools: false,
            defaultViewport: {
                width: 1920,
                height: 1080,
            },
        });

        // Open a new page
        const page = await browser.newPage();

        const cookiesAvailable = await fileExists('./cookies.json');

        if (!cookiesAvailable) {

            // Navigate to the page below
            await page.goto(myArgs[0]);

            // Log the cookies
            const cookies = await page.cookies();
            const cookieJson = JSON.stringify(cookies);
            writeFileSync('cookies.json', cookieJson);

            // Close the browser
            return await browser.close();
        }

        // Set Cookies
        const cookies = readFileSync('cookies.json', 'utf8');
        const deserializedCookies = JSON.parse(cookies);
        await page.setCookie(...deserializedCookies);

        // Navigate to the page below
        await page.goto(myArgs[0]);

        await page.waitForTimeout(5000);

        // Determin current URL
        const currentURL = page.url();

        console.log(`Gathering Info: ${currentURL}`);

        // In browser code
        const reviewPageUrls = await page.evaluate(() => {

            // All review count
            // let totalReviewCount = parseInt(document.querySelectorAll("a[href='#REVIEWS']")[1].innerText.split('\n')[1].split(' ')[0].replace(',', ''))

            // English review count
            let totalReviewCount = parseInt(document.getElementsByClassName('ui_radio dQNlC')[1].innerText.split('(')[1].split(')')[0].replace(',', ''))

            // Calculate the last review page
            totalReviewCount = (totalReviewCount - totalReviewCount % 5) / 5

            // Get the url format
            const url = document.getElementsByClassName('pageNum')[1].href

            return { totalReviewCount, url }

        })

        // Destructure function outputs
        let { totalReviewCount, url } = reviewPageUrls;

        console.log(reviewPageUrls)
        // Array to hold all the review urls
        const allUrls = []

        let counter = 0
        // Replace the url page count till the last page
        while (counter < totalReviewCount) {
            counter++
            url = url.replace(/-or[0-9]*/g, `-or${counter * 5}`)
            allUrls.push(url)
        }


        // JSON structure
        const data = {
            count: allUrls.length * 5,
            pageCount: allUrls.length,
            urls: allUrls
        };

        // Write the data to a json file
        writeFileSync('reviewUrl.json', JSON.stringify(data));

        return allUrls

    } catch (err) {
        throw err
    }
}


const start = async () => {
    try {

        const allReviewsUrl = await extractAllReviewPageUrls();
        const results = await scrap(allReviewsUrl);
        const csv = parse(myData, opts);
        writeFileSync('./review.csv', csv);
    } catch (err) {
        throw err;
    }
}
start().catch(err => console.error(err));