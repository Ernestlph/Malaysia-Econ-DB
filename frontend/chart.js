// --- DOM Elements ---
const priceChartContainer = document.getElementById('price-chart-container');
const returnChartContainer = document.getElementById('return-chart-container');
const loadingIndicators = document.querySelectorAll('.loading-indicator');
const dataTypeSelect = document.getElementById('dataType');
const dataCodeInput = document.getElementById('dataCode');
const addButton = document.getElementById('addButton');
const startDateInput = document.getElementById('startDate');
const endDateInput = document.getElementById('endDate');
const reloadButton = document.getElementById('reloadButton');
const activeSeriesListDiv = document.getElementById('activeSeriesList');
const statsDisplayDiv = document.getElementById('statsDisplay');
const customTooltipDiv = document.getElementById('custom-tooltip');

// --- Chart Instances ---
let priceChart = null;
let returnChart = null;

// --- State Management ---
// Stores info about series currently displayed { id: { type, code, name, companyName, data, color, priceScaleId, seriesRefs: {price, sma5, sma20, return} } }
let activeSeries = {};
let nextPriceScaleId = 'right'; // Alternate 'left'/'right' for price scales

// --- Default Dates ---
const today = new Date();
const oneYearAgo = new Date();
oneYearAgo.setFullYear(today.getFullYear() - 1);
endDateInput.value = today.toISOString().split('T')[0];
startDateInput.value = oneYearAgo.toISOString().split('T')[0];

// --- Series Type Mapping for Lightweight Charts addSeries method ---
const seriesDefinitionMap = {
    'stock': () => LightweightCharts.LineSeries, // Assuming you want LineSeries for stocks
    'fx': () => LightweightCharts.LineSeries,    // Assuming LineSeries for FX

};

// --- Calculation Functions ---
function calculateSMA(priceData, period) {
    if (!priceData || priceData.length < period) {
        return [];
    }
    const smaValues = [];
    // Calculate initial sum for the first SMA value
    let currentSum = 0;
    for (let i = 0; i < period; i++) {
        currentSum += priceData[i].value;
    }
    smaValues.push(currentSum / period);

    // Calculate subsequent SMA values efficiently
    for (let i = period; i < priceData.length; i++) {
        currentSum -= priceData[i - period].value; // Subtract the oldest value
        currentSum += priceData[i].value;         // Add the newest value
        smaValues.push(currentSum / period);
    }

    // Map calculated values back to time points, padding the start
    const result = [];
    for (let i = 0; i < priceData.length; i++) {
        if (i < period - 1) {
            result.push({ time: priceData[i].time, value: undefined }); // Padding
        } else {
            result.push({ time: priceData[i].time, value: smaValues[i - (period - 1)] });
        }
    }
    return result;
}

/**
 * Calculates Total Return Percentage over the period.
 * The first data point in the selected range is the baseline (0% change).
 * @param {Array<{time: string, value: number}>} priceData - Formatted price data
 * @returns {Array<{time: string, value: number|undefined}>} - Total Return % data points
 */
function calculateTotalReturn(priceData) {
    if (!priceData || priceData.length < 1) {
        return [];
    }
    const startPrice = priceData[0].value; // First price in the *selected range*
    if (startPrice <= 0) { // Handle edge cases like zero or negative start price
        console.warn("Cannot calculate total return with non-positive or zero starting price. Returning undefined for all points.");
        return priceData.map(point => ({ time: point.time, value: undefined }));
    }

    return priceData.map(point => ({
        time: point.time,
        value: ((point.value - startPrice) / startPrice) * 100, // Percentage change from the startPrice
    }));
}

function calculateCorrelation(arr1, arr2) {
    if (arr1.length !== arr2.length || arr1.length < 2) return NaN;
    const n = arr1.length;
    const sumX = arr1.reduce((a, b) => a + b, 0);
    const sumY = arr2.reduce((a, b) => a + b, 0);
    const meanX = sumX / n;
    const meanY = sumY / n;
    let sumXY = 0, sumX2 = 0, sumY2 = 0;
    for (let i = 0; i < n; i++) {
        sumXY += (arr1[i] - meanX) * (arr2[i] - meanY);
        sumX2 += Math.pow(arr1[i] - meanX, 2);
        sumY2 += Math.pow(arr2[i] - meanY, 2);
    }
    const denominator = Math.sqrt(sumX2 * sumY2);
    return denominator === 0 ? 0 : sumXY / denominator; // Return 0 if no variance
}


// --- Helper Functions ---
function setLoading(isLoading) {
    loadingIndicators.forEach(el => el.style.display = isLoading ? 'block' : 'none');
}

function generateSeriesId(type, code) {
    return `${type}_${code}`;
}

const colorPalette = ['#2962FF', '#E91E63', '#4CAF50', '#9C27B0', '#F44336', '#795548', '#607D8B'];
let colorIndex = 0;
function getNextColor() {
    const color = colorPalette[colorIndex % colorPalette.length];
    colorIndex++;
    return color;
}

// Helper function to lighten/darken a hex color
// Amount: positive to lighten, negative to darken
function adjustColor(color, amount) {
    let usePound = false;
    if (color[0] === "#") {
        color = color.slice(1);
        usePound = true;
    }
    const num = parseInt(color, 16);
    let r = (num >> 16) + amount;
    if (r > 255) r = 255; else if (r < 0) r = 0;
    let g = ((num >> 8) & 0x00FF) + amount; // Corrected: was b, should be g
    if (g > 255) g = 255; else if (g < 0) g = 0;
    let b = (num & 0x0000FF) + amount; // Corrected: was g, should be b
    if (b > 255) b = 255; else if (b < 0) b = 0;

    // Ensure 6 digits by padding with leading zeros if necessary
    const R = r.toString(16).padStart(2, '0');
    const G = g.toString(16).padStart(2, '0');
    const B = b.toString(16).padStart(2, '0');

    return (usePound ? "#" : "") + R + G + B;
}

function getNextPriceScaleId() {
    const current = nextPriceScaleId;
    nextPriceScaleId = (current === 'right') ? 'left' : 'right';
    return current;
}

const apiEndpoints = {
    stock: (code) => `/api/stock/prices?code=${code}`,
    fx: (code) => `/api/fx/rates?code=${code}`,
    // loan: (code) => `/api/loans/sector?sector_id=${code}`, // Example for future
};

// --- Helper to format date as "MMM-DD-YY" ---
function formatTooltipDate(timestamp) {
    const date = new Date(timestamp * 1000); // Lightweight Charts time is UNIX timestamp
    const options = { month: 'short', day: 'numeric', year: '2-digit' };
    return date.toLocaleDateString('en-US', options).replace(/, /g, '-');
}


// --- Data Fetching ---
async function fetchSeriesData(type, code, startDate, endDate) {
    const endpointBuilder = apiEndpoints[type];
    if (!endpointBuilder) throw new Error(`Unknown data type: ${type}`);
    const apiUrl = `${endpointBuilder(code)}&start_date=${startDate}&end_date=${endDate}`;
    console.log(`Fetching: ${apiUrl}`);

    const response = await fetch(apiUrl);
    if (!response.ok) {
        const errorText = await response.text();
        console.error(`API Error Response Body for ${type} ${code}:`, errorText);
        throw new Error(`HTTP error for ${type} ${code}! Status: ${response.status}. Check console.`);
    }
    const rawData = await response.json();

    if (!Array.isArray(rawData)) {
        console.error(`Invalid data format received for ${type} ${code}:`, rawData);
        throw new Error(`Data for ${type} ${code} is not an array.`);
    }
    console.log(`Received ${rawData.length} points for ${type} ${code}`);

    let fetchedCompanyName = code; // Default to code
    // Assuming your API for stock prices now returns company_name
    if (type === 'stock' && rawData.length > 0 && rawData[0].company_name) {
        fetchedCompanyName = rawData[0].company_name;
    }

    const processedData = rawData
        .map(item => ({
            time: item.date, // Expects YYYY-MM-DD
            value: typeof item.value === 'number' ? item.value : parseFloat(item.value)
        }))
        .filter(item => item.time && !isNaN(item.value)) // Filter out invalid times or numbers
        .sort((a, b) => new Date(a.time) - new Date(b.time));

    return {
        dataPoints: processedData,
        companyName: fetchedCompanyName,
    };
}


// --- Chart Rendering ---
function renderCharts() {
    setLoading(true);
    if (priceChart) { priceChart.remove(); priceChart = null; }
    if (returnChart) { returnChart.remove(); returnChart = null; }
    priceChartContainer.innerHTML = '';
    returnChartContainer.innerHTML = '';
    Object.values(activeSeries).forEach(s => s.seriesRefs = {});

    if (Object.keys(activeSeries).length === 0) {
        priceChartContainer.innerHTML = 'Add a series to begin.';
        if (statsDisplayDiv) statsDisplayDiv.innerHTML = '';
        setLoading(false);
        return;
    }

    try {
        // DEBUGGING LOGS for chart creation
        console.log("DEBUG: About to create charts. LightweightCharts object:", typeof LightweightCharts !== 'undefined' ? LightweightCharts : 'UNDEFINED');
        console.log("DEBUG: priceChartContainer element:", priceChartContainer ? 'Found' : 'MISSING');
        console.log("DEBUG: returnChartContainer element:", returnChartContainer ? 'Found' : 'MISSING');

        if (!priceChartContainer || !returnChartContainer) throw new Error("One or both chart container elements not found.");
        if (typeof LightweightCharts === 'undefined' || !LightweightCharts.createChart) throw new Error("LightweightCharts library or createChart function not available.");

        priceChart = LightweightCharts.createChart(priceChartContainer, {
            width: priceChartContainer.clientWidth, height: priceChartContainer.clientHeight,
            layout: { background: { color: '#ffffff' }, textColor: '#333' },
            grid: { vertLines: { color: '#e1e1e1' }, horzLines: { color: '#e1e1e1' } },
            // --- Crosshair options for tooltip ---
            crosshair: {
                mode: LightweightCharts.CrosshairMode.Normal,
                // We'll build our own tooltip, so hide default labels if too noisy
                vertLine: { labelVisible: true, style: LightweightCharts.LineStyle.Dashed },
                horzLine: { labelVisible: true, style: LightweightCharts.LineStyle.Dashed },
            },
            timeScale: { timeVisible: true, secondsVisible: false, borderColor: '#e1e1e1' },
            leftPriceScale: { visible: true, borderColor: '#e1e1e1' },
            rightPriceScale: { visible: true, borderColor: '#e1e1e1' },
        });
        console.log("DEBUG: Result of createChart for priceChart:", priceChart ? 'Object created' : 'FAILED');

        returnChart = LightweightCharts.createChart(returnChartContainer, {
            width: returnChartContainer.clientWidth, height: returnChartContainer.clientHeight,
            layout: { background: { color: '#ffffff' }, textColor: '#333' },
            grid: { vertLines: { color: '#e1e1e1' }, horzLines: { color: '#e1e1e1' } },
            crosshair: {
                mode: LightweightCharts.CrosshairMode.Normal,
                vertLine: { labelVisible: true, style: LightweightCharts.LineStyle.Dashed },
                horzLine: { labelVisible: true, style: LightweightCharts.LineStyle.Dashed },
            },
            timeScale: { timeVisible: true, secondsVisible: false, borderColor: '#e1e1e1' },
            rightPriceScale: { borderColor: '#e1e1e1', scaleMargins: { top: 0.2, bottom: 0.1 } },
        });
        console.log("DEBUG: Result of createChart for returnChart:", returnChart ? 'Object created' : 'FAILED');

        if (!priceChart || typeof priceChart.addSeries !== 'function') throw new Error("createChart for priceChart invalid.");
        if (!returnChart || typeof returnChart.addSeries !== 'function') throw new Error("createChart for returnChart invalid.");

    } catch (chartError) {
        console.error("CRITICAL: Error creating chart instance(s):", chartError);
        priceChartContainer.innerHTML = `<div style="color: red; padding: 20px;">Error creating charts: ${chartError.message}.</div>`;
        if (statsDisplayDiv) statsDisplayDiv.innerHTML = '';
        setLoading(false); priceChart = null; returnChart = null; return;
    }

    let firstSeriesId = null;
    for (const id in activeSeries) {
        const seriesInfo = activeSeries[id];
        if (!seriesInfo.data || seriesInfo.data.length === 0) continue;
        if (!firstSeriesId) firstSeriesId = id;

        const sma5Data = calculateSMA(seriesInfo.data, 5);
        const sma20Data = calculateSMA(seriesInfo.data, 20);
        const totalReturnData = calculateTotalReturn(seriesInfo.data);
        const seriesTitle = seriesInfo.name || seriesInfo.code; // Use companyName or fallback to code

        try {
            const getSeriesDef = seriesDefinitionMap[seriesInfo.type];
            if (!getSeriesDef) { console.error(`No series definition for type: ${seriesInfo.type}`); continue; }
            const seriesConstructor = getSeriesDef(); // This should return e.g., LightweightCharts.LineSeries

            seriesInfo.seriesRefs.price = priceChart.addSeries(seriesConstructor, {
                color: seriesInfo.color,
                lineWidth: 2,
                title: seriesTitle,
                priceScaleId: seriesInfo.priceScaleId
            });
            seriesInfo.seriesRefs.price.setData(seriesInfo.data);

            const derivedSma5Color = adjustColor(seriesInfo.color, 60);  // Lighten main color
            const derivedSma20Color = adjustColor(seriesInfo.color, -40); // Darken main color (or use a different offset)


            seriesInfo.seriesRefs.sma5 = priceChart.addSeries(LightweightCharts.LineSeries, {
                color: derivedSma5Color,
                lineWidth: 1,
                lineStyle: LightweightCharts.LineStyle.Dashed, // Different style for SMA5
                title: `SMA5(${seriesTitle})`,
                priceScaleId: seriesInfo.priceScaleId,
                lastValueVisible: false, priceLineVisible: false,
            });
            seriesInfo.seriesRefs.sma5.setData(sma5Data.filter(d => d.value !== undefined));

            seriesInfo.seriesRefs.sma20 = priceChart.addSeries(LightweightCharts.LineSeries, {
                color: derivedSma20Color,
                lineWidth: 1,
                lineStyle: LightweightCharts.LineStyle.Dotted, // Different style for SMA20
                title: `SMA20(${seriesTitle})`,
                priceScaleId: seriesInfo.priceScaleId,
                lastValueVisible: false, priceLineVisible: false,
            });
            seriesInfo.seriesRefs.sma20.setData(sma20Data.filter(d => d.value !== undefined));


            seriesInfo.seriesRefs.return = returnChart.addSeries(LightweightCharts.LineSeries, {
                color: seriesInfo.color,
                lineWidth: 1,
                title: `${seriesTitle} Ret%`,
                priceFormat: { type: 'percent', precision: 2, minMove: 0.01 },
                lastValueVisible: true, priceLineVisible: true,
            });
            seriesInfo.seriesRefs.return.setData(totalReturnData.filter(d => d.value !== undefined));
        } catch (seriesError) {
            console.error(`Error adding series data for ${id}:`, seriesError);
        }
    }

    // --- Correlation Calculation and Display ---
    if (statsDisplayDiv) statsDisplayDiv.innerHTML = '';
    if (Object.keys(activeSeries).length === 2) {
        const seriesIds = Object.keys(activeSeries);
        const series1Info = activeSeries[seriesIds[0]];
        const series2Info = activeSeries[seriesIds[1]];
        const returnData1 = calculateTotalReturn(series1Info.data); // Recalculate or use stored if available
        const returnData2 = calculateTotalReturn(series2Info.data);
        const pairedValues1 = [], pairedValues2 = [];
        const returnData2Map = new Map(returnData2.filter(p => p.value !== undefined && !isNaN(p.value)).map(p => [p.time, p.value]));
        returnData1.forEach(p1 => {
            if (p1.value !== undefined && !isNaN(p1.value)) {
                const p2Value = returnData2Map.get(p1.time);
                if (p2Value !== undefined) {
                    pairedValues1.push(p1.value);
                    pairedValues2.push(p2Value);
                }
            }
        });
        if (pairedValues1.length > 1) {
            const correlation = calculateCorrelation(pairedValues1, pairedValues2);
            if (statsDisplayDiv && !isNaN(correlation)) {
                const name1 = series1Info.name || series1Info.code;
                const name2 = series2Info.name || series2Info.code;
                statsDisplayDiv.innerHTML = `<strong>Correlation (${name1} vs ${name2} Returns):</strong> ${correlation.toFixed(4)}`;
            }
        } else {
            if (statsDisplayDiv) statsDisplayDiv.innerHTML = `<strong>Correlation:</strong> Not enough overlapping data.`;
        }
    }

    // --- Synchronize Charts ---
    // --- Tooltip and Synchronization Logic ---
    function updateTooltip(chartInstance, param, allSeriesInfo) {
        if (!param.time || !param.point || !customTooltipDiv || !chartInstance) {
            customTooltipDiv.style.display = 'none';
            return;
        }

        let tooltipHtml = `<div style="font-weight: bold; margin-bottom: 5px;">${formatTooltipDate(param.time)}</div>`;
        let contentAdded = false;

        // Iterate over all series associated with THIS chart instance
        for (const id in allSeriesInfo) {
            const seriesInfo = allSeriesInfo[id];
            const seriesObject = param.seriesData.get(seriesInfo.seriesRefs.price) || // Check main price series
                param.seriesData.get(seriesInfo.seriesRefs.sma5) ||
                param.seriesData.get(seriesInfo.seriesRefs.sma20) ||
                param.seriesData.get(seriesInfo.seriesRefs.return); // Check return series

            if (seriesObject && seriesObject.value !== undefined) { // Check if data exists for this series at this time
                const seriesTitle = seriesInfo.name || seriesInfo.code;
                let value = seriesObject.value;
                let valueStr;

                // Check if this series is from the returnChart to format as percentage
                if (chartInstance === returnChart && seriesInfo.seriesRefs.return && param.seriesData.has(seriesInfo.seriesRefs.return)) {
                    valueStr = `${value.toFixed(2)}%`;
                } else {
                    valueStr = typeof value === 'number' ? value.toFixed(2) : value; // Adjust precision as needed for price/SMAs
                }

                tooltipHtml += `
                    <div style="display: flex; align-items: center; margin-bottom: 3px;">
                        <span style="width: 10px; height: 10px; background-color: ${seriesInfo.color}; margin-right: 5px; display: inline-block;"></span>
                        <span>${seriesTitle}:</span>
                        <span style="margin-left: auto; font-weight: bold;">${valueStr}</span>
                    </div>`;
                contentAdded = true;
            }
        }


        if (contentAdded) {
            customTooltipDiv.innerHTML = tooltipHtml;
            customTooltipDiv.style.display = 'block';

            // Position the tooltip
            // Get container's bounding rectangle to position tooltip relative to it
            const chartRect = chartInstance.chartElement().getBoundingClientRect();
            const tooltipRect = customTooltipDiv.getBoundingClientRect();

            let left = param.point.x + chartRect.left + 15; // 15px offset from crosshair
            let top = param.point.y + chartRect.top + 15;

            // Adjust if tooltip goes off-screen
            if (left + tooltipRect.width > window.innerWidth) {
                left = param.point.x + chartRect.left - tooltipRect.width - 15;
            }
            if (top + tooltipRect.height > window.innerHeight) {
                top = param.point.y + chartRect.top - tooltipRect.height - 15;
            }
            // Ensure it doesn't go off the top/left of the viewport
            if (top < 0) top = 5;
            if (left < 0) left = 5;


            customTooltipDiv.style.left = `${left}px`;
            customTooltipDiv.style.top = `${top}px`;
        } else {
            customTooltipDiv.style.display = 'none';
        }
    }


    if (priceChart && returnChart) {
        // Time Scale Sync
        priceChart.timeScale().subscribeVisibleLogicalRangeChange(range => { if (returnChart && returnChart.timeScale()) returnChart.timeScale().setVisibleLogicalRange(range); });
        returnChart.timeScale().subscribeVisibleLogicalRangeChange(range => { if (priceChart && priceChart.timeScale()) priceChart.timeScale().setVisibleLogicalRange(range); });

        // Crosshair Sync & Tooltip Update
        let isSyncing = false;
        priceChart.subscribeCrosshairMove(param => {
            if (isSyncing) return;
            isSyncing = true;
            if (returnChart && param.point) returnChart.moveCrosshair(param.point);
            updateTooltip(priceChart, param, activeSeries); // Update tooltip for price chart
            if (returnChart && param.time) { // Also update/show tooltip for return chart if hovered on price chart
                const returnParam = { ...param, seriesData: new Map() }; // Create a new param for return chart context
                for (const id in activeSeries) {
                    const seriesInfo = activeSeries[id];
                    if (seriesInfo.seriesRefs.return) {
                        const dataPoint = param.seriesData.get(seriesInfo.seriesRefs.return); // This might be incorrect if not all series are on price chart
                        // We need to get data from the return series itself
                        const returnDataAtTime = findDataAtTime(seriesInfo.seriesRefs.return.data(), param.time); // Need a helper
                        if (returnDataAtTime) {
                            returnParam.seriesData.set(seriesInfo.seriesRefs.return, returnDataAtTime);
                        }
                    }
                }
                // updateTooltip(returnChart, returnParam, activeSeries); // Could be noisy, decide if needed
            }
            isSyncing = false;
        });

        returnChart.subscribeCrosshairMove(param => {
            if (isSyncing) return;
            isSyncing = true;
            if (priceChart && param.point) priceChart.moveCrosshair(param.point);
            updateTooltip(returnChart, param, activeSeries); // Update tooltip for return chart
            if (priceChart && param.time) { // Also update/show tooltip for price chart if hovered on return chart
                const priceParam = { ...param, seriesData: new Map() };
                for (const id in activeSeries) {
                    const seriesInfo = activeSeries[id];
                    if (seriesInfo.seriesRefs.price) {
                        const priceDataAtTime = findDataAtTime(seriesInfo.seriesRefs.price.data(), param.time);
                        if (priceDataAtTime) {
                            priceParam.seriesData.set(seriesInfo.seriesRefs.price, priceDataAtTime);
                        }
                    }
                }
                // updateTooltip(priceChart, priceParam, activeSeries); // Could be noisy
            }
            isSyncing = false;
        });

        // Hide tooltip when mouse leaves chart area
        priceChartContainer.addEventListener('mouseleave', () => { customTooltipDiv.style.display = 'none'; });
        returnChartContainer.addEventListener('mouseleave', () => { customTooltipDiv.style.display = 'none'; });
    }

    if (priceChart && returnChart && firstSeriesId && activeSeries[firstSeriesId]) {
        priceChart.timeScale().subscribeVisibleLogicalRangeChange(range => { if (returnChart && returnChart.timeScale()) returnChart.timeScale().setVisibleLogicalRange(range); });
        returnChart.timeScale().subscribeVisibleLogicalRangeChange(range => { if (priceChart && priceChart.timeScale()) priceChart.timeScale().setVisibleLogicalRange(range); });
        let isSyncing = false;
        priceChart.subscribeCrosshairMove(param => {
            if (isSyncing || !returnChart || !param || !param.time || !activeSeries[firstSeriesId] || !activeSeries[firstSeriesId].seriesRefs.return) return;
            isSyncing = true; returnChart.moveCrosshair(param.point); isSyncing = false;
        });
        returnChart.subscribeCrosshairMove(param => {
            if (isSyncing || !priceChart || !param || !param.time || !activeSeries[firstSeriesId] || !activeSeries[firstSeriesId].seriesRefs.price) return;
            isSyncing = true; priceChart.moveCrosshair(param.point); isSyncing = false;
        });
    }

    if (priceChart) priceChart.timeScale().fitContent();
    if (returnChart) returnChart.timeScale().fitContent();

    updateActiveSeriesListUI();
    setLoading(false);
    console.log("DEBUG: renderCharts completed.");
}

// Helper function to find data point at a specific time (binary search for performance if needed)
// For simplicity, using a linear scan here. Assumes seriesData is sorted by time.
function findDataAtTime(seriesDataArray, time) {
    if (!seriesDataArray || seriesDataArray.length === 0) return undefined;
    // Lightweight Charts time is a UNIX timestamp. Data in seriesDataArray is {time: 'YYYY-MM-DD', value: ...}
    // We need to convert 'YYYY-MM-DD' to a comparable timestamp or find the closest match.
    // For this example, let's assume the 'time' in crosshair param is directly usable
    // or we'd need a more complex lookup based on date string.
    // The `param.seriesData.get(series)` is the more direct way if the series is on *that* chart.
    for (const point of seriesDataArray) {
        // Convert point.time (YYYY-MM-DD string) to a comparable format if param.time is a timestamp
        // Or, if param.time is a business day object, use its timestamp.
        // This part is tricky without knowing the exact format of param.time vs point.time
        const pointDate = new Date(point.time + 'T00:00:00Z'); // Treat as UTC start of day
        const paramDate = new Date(time * 1000); // Convert UNIX timestamp to Date

        if (pointDate.getUTCFullYear() === paramDate.getUTCFullYear() &&
            pointDate.getUTCMonth() === paramDate.getUTCMonth() &&
            pointDate.getUTCDate() === paramDate.getUTCDate()) {
            return point;
        }
    }
    return undefined;
}

// --- UI Update Functions ---
function updateActiveSeriesListUI() {
    activeSeriesListDiv.innerHTML = '<strong>Active Series:</strong><br>';
    if (Object.keys(activeSeries).length === 0) {
        activeSeriesListDiv.innerHTML += '<i>None added yet.</i>'; return;
    }
    for (const id in activeSeries) {
        const seriesInfo = activeSeries[id];
        const displayName = seriesInfo.name || seriesInfo.code;
        const itemDiv = document.createElement('div');
        itemDiv.classList.add('active-series-item');
        itemDiv.innerHTML = `<span style="background-color:${seriesInfo.color}; width: 12px; height: 12px; display: inline-block; margin-right: 8px; border: 1px solid #ccc;"></span> ${displayName} (${seriesInfo.type.toUpperCase()}) - Scale: ${seriesInfo.priceScaleId} <span class="remove-btn" data-id="${id}" title="Remove Series">✖</span>`;
        itemDiv.querySelector('.remove-btn').addEventListener('click', (e) => removeSeries(e.target.dataset.id));
        activeSeriesListDiv.appendChild(itemDiv);
    }
}

// --- Add/Remove Series Logic ---
async function addSeries() {
    const type = dataTypeSelect.value;
    const code = dataCodeInput.value.trim().toUpperCase();
    const startDate = startDateInput.value;
    const endDate = endDateInput.value;
    if (!code) { alert('Please enter a code/ID.'); return; }
    if (!startDate || !endDate) { alert('Please select a start and end date.'); return; }
    const seriesId = generateSeriesId(type, code);
    if (activeSeries[seriesId]) { alert(`${code} (${type}) is already added.`); return; }
    if (!seriesDefinitionMap[type]) { alert(`Data type '${type}' is not supported.`); return; }

    setLoading(true);
    try {
        const fetchedResult = await fetchSeriesData(type, code, startDate, endDate);
        if (fetchedResult.dataPoints.length === 0) {
            alert(`No data found for ${code} (${type}).`);
        } else {
            activeSeries[seriesId] = {
                type: type,
                code: code,
                name: fetchedResult.companyName, // Store company name from fetch
                data: fetchedResult.dataPoints,
                color: getNextColor(),
                priceScaleId: getNextPriceScaleId(),
                seriesRefs: {}
            };
            dataCodeInput.value = '';
            renderCharts();
        }
    } catch (error) {
        console.error(`Error adding series ${seriesId}:`, error);
        alert(`Failed to add series ${code} (${type}): ${error.message}.`);
    } finally { setLoading(false); }
}

function removeSeries(seriesId) {
    if (activeSeries[seriesId]) {
        console.log(`INFO: Removing series: ${seriesId}`);
        const refs = activeSeries[seriesId].seriesRefs;
        // Ensure charts exist before trying to remove series from them
        if (priceChart) {
            if (refs.price) try { priceChart.removeSeries(refs.price); } catch (e) { console.warn("Error removing price series:", e); }
            if (refs.sma5) try { priceChart.removeSeries(refs.sma5); } catch (e) { console.warn("Error removing sma5 series:", e); }
            if (refs.sma20) try { priceChart.removeSeries(refs.sma20); } catch (e) { console.warn("Error removing sma20 series:", e); }
        }
        if (returnChart && refs.return) {
            try { returnChart.removeSeries(refs.return); } catch (e) { console.warn("Error removing return series:", e); }
        }
        delete activeSeries[seriesId];
        renderCharts();
    }
}

// --- Event Listeners ---
addButton.addEventListener('click', addSeries);
reloadButton.addEventListener('click', () => {
    console.log("INFO: Reload All button clicked.");
    const currentStartDate = startDateInput.value;
    const currentEndDate = endDateInput.value;
    if (!currentStartDate || !currentEndDate) { alert("Ensure dates are selected."); return; }

    const seriesToReload = { ...activeSeries }; // Keep existing series info (color, scaleId etc.)
    activeSeries = {}; // Clear current data and seriesRefs
    // colorIndex = 0; // Optionally reset color cycle, or let it continue
    // nextPriceScaleId = 'right'; // Optionally reset scale cycle

    if (Object.keys(seriesToReload).length > 0) {
        setLoading(true);
        let promises = Object.values(seriesToReload).map(info =>
            fetchSeriesData(info.type, info.code, currentStartDate, currentEndDate)
                .then(fetchedResult => ({ // Pass existing info and new data
                    id: generateSeriesId(info.type, info.code),
                    type: info.type,
                    code: info.code,
                    name: fetchedResult.companyName, // Update name with potentially new fetched name
                    data: fetchedResult.dataPoints,
                    color: info.color, // Keep original color
                    priceScaleId: info.priceScaleId, // Keep original scale assignment
                    error: null
                }))
                .catch(error => ({
                    id: generateSeriesId(info.type, info.code),
                    type: info.type,
                    code: info.code,
                    name: info.name, // Keep original name
                    data: [],
                    error: error
                }))
        );
        Promise.all(promises).then(results => {
            results.forEach(res => {
                if (res.error) {
                    console.error(`Error reloading data for ${res.id}:`, res.error);
                    alert(`Failed to reload data for ${res.code} (${res.type}). Check console.`);
                } else if (res.data.length > 0) {
                    console.log(`INFO: Successfully reloaded data for ${res.id}`);
                    activeSeries[res.id] = { ...res, seriesRefs: {} }; // Rebuild state object
                } else {
                    console.warn(`No data found on reload for ${res.id}`);
                }
            });
            renderCharts();
        }).finally(() => setLoading(false));
    } else {
        renderCharts();
    }
});

// --- Initial Load ---
updateActiveSeriesListUI();
renderCharts(); // Render initial empty state

// --- Resize Handler ---
let resizeTimeout;
window.addEventListener('resize', () => {
    clearTimeout(resizeTimeout);
    resizeTimeout = setTimeout(() => {
        console.log("DEBUG: Resizing charts.");
        if (priceChart) try { priceChart.resize(priceChartContainer.clientWidth, priceChartContainer.clientHeight); } catch (e) { console.error("Resize price chart error:", e); }
        if (returnChart) try { returnChart.resize(returnChartContainer.clientWidth, returnChartContainer.clientHeight); } catch (e) { console.error("Resize return chart error:", e); }
    }, 200);
});