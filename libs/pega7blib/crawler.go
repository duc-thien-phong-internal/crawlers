package pega7lib

import (
	"encoding/json"
	"fmt"
	"github.com/antchfx/htmlquery"
	"github.com/duc-thien-phong-internal/crawlers/pkg/basecrawler"
	"github.com/duc-thien-phong-internal/crawlers/pkg/my_selenium"
	"github.com/duc-thien-phong/techsharedservices/logger"
	"github.com/duc-thien-phong/techsharedservices/models/customer"
	"github.com/duc-thien-phong/techsharedservices/utils"
	"github.com/tebeka/selenium"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Crawler struct {
	Base
}

// "//*[@id='iconExpandCollapse']/a[@id='fnt_GrandTotal']"
// number of apps: "//tr[@id='GrandTotal']/td[2]"

func NewCrawler(withDocker bool, args map[string]interface{}) *Crawler {
	p := Crawler{}
	p.SetNeedToBeReplaced(false)

	browser := my_selenium.CreateBrowser(withDocker, 24*60*60, 24*60*60, args)
	if browser == nil {
		logger.Root.Errorf("Could not create selenium browser")
		return nil
	} else {
		logger.Root.Infof("Browser is created!")
	}
	p.Browser = browser

	return &p
}

func (p *Crawler) Prepare() error {
	return nil
}

func (p *Crawler) getMaxLifeDuration() int {
	N := 4 * 60 // 4 hours
	if d, ok := p.ClientAdapter.GetHostApplication().GetWorkerConfig().OtherConfig["crawlerMaxLifeDuration"]; ok {
		if v, ok := d.(float64); ok {
			N = int(v)
		} else {
			logger.Root.Infof("Could not convert crawlerMaxLifeDuration %T to int", d)
		}
	}
	return N
}

func (p *Crawler) Run(wg *sync.WaitGroup) {
	for {
		if p.GetNeedToBeReplaced() || p.NeedToBeClosed() || p.ClientAdapter.GetNeedToCloseCrawlers() || !p.ClientAdapter.CheckIfIsInCrawlerList(p.GetID()) {
			logger.Root.Infof("browser: `%s` needToBeReplace or needToClose", p.GetID())
			break
		}

		wg.Add(1)
		// p.adapter.chanCrawlers <- struct{}{}
		p.startCrawling(wg)
		if p.GetNeedToBeReplaced() || p.NeedToBeClosed() || p.ClientAdapter.GetNeedToCloseCrawlers() || !p.ClientAdapter.CheckIfIsInCrawlerList(p.GetID()) {
			logger.Root.Infof("browser: `%s` needToBeReplace or needToClose", p.GetID())
			break
		}
	}
	p.ClientAdapter.RemoveCrawler(p.GetID())

	p.Close()
}

func (p *Crawler) ProcessPage(config *basecrawler.Config, pageSize int) (workIDs []string, hasNextPage bool, err error) {
	productOptions := ""
	if d, ok := p.ClientAdapter.GetHostApplication().GetWorkerConfig().OtherConfig["productOptions"]; ok {
		if v, ok := d.(string); ok && len(strings.Trim(v, " \n\t")) > 0 {
			productOptions = v
		}
	}

	var pageContent string
	pageContent, hasNextPage, err = p.fetchPageWithBrowser(p.ClientAdapter.GetHostApplication().GetConfig().CurrentCrawlingPage, pageSize)
	if err != nil {
		logger.Root.Infof("`%s` Sleep 5 seconds before retrying to fetch page %d", p.GetID(), p.ClientAdapter.GetHostApplication().GetConfig().CurrentCrawlingPage)
		// if after a try, we still could not set the start date, maybe the producer is unusable anymore
		if err = p.setTheStartDayToCrawl(productOptions); err != nil {
			err = basecrawler.ErrInvalidProducer
			return
		}
		time.Sleep(5 * time.Second)
		return
	}
	workIDs = p.parsePageToGetWorkIDs("<html>" + pageContent + "</html>")
	if len(workIDs) == 0 && hasNextPage {
		logger.Root.Warnf("Something weird happen. There is no workids but we have the next page")
	}
	return
}

func (p *Crawler) startCrawling(wg *sync.WaitGroup) {
	defer wg.Done()
	defer func() {
		(*p.Browser.DriverInfo.Driver).ExecuteScript("document.title = '"+p.GetID()+"'", []interface{}{})
	}()
	defer func() {
		if r := recover(); r != nil {
			logger.Root.Infof("Recover from startCrawling of `%s`. Err: %v", p.GetID(), r)
		}
	}()
	(*p.Browser.DriverInfo.Driver).ExecuteScript("document.title = 'Crawler - "+p.GetID()+"'", []interface{}{})
	// defer func() {
	// 	p.adapter.chanCrawlers <- struct{}{}
	// }()

	logger.Root.Infof("Browser `%s` (idx:%d) start crawling....", p.GetID(), p.ClientAdapter.GetNumRunningCrawlers())
	timer := time.NewTimer(time.Duration(p.getMaxLifeDuration()+rand.Intn(10)) * time.Minute)
	defer timer.Stop()
	var nedToStopBecauseOfError bool

	adapter, _ := (p.ClientAdapter).(*Adapter)
mainFor:
	for !nedToStopBecauseOfError && !p.GetNeedToBeReplaced() && !p.NeedToBeClosed() && !p.ClientAdapter.GetNeedToCloseCrawlers() {
		if !p.ClientAdapter.CheckIfIsInCrawlerList(p.GetID()) {
			// I don't belong to the crawler list anymore
			// maybe I am promoted to be a producer
			break
		}
		select {
		case <-timer.C:
			p.SetNeedToBeReplaced(true)
			timer.Stop()
			logger.Root.Infof("Set crawler `%s` to be replaced", p.GetID())
			break mainFor
		case meta := <-adapter.NeedToCrawled:
			adapter.AddNumNeedToCrawl(-1)
			if meta.DataID == "" {
				break mainFor
			}

			logger.Root.Infof("Browser `%s` Processes app %v", p.GetID(), meta.DataID)

			if adapter.CheckIfExtractedBefore(meta.DataID) || meta.Tried >= 7 {
				logger.Root.With("crawlerID", p.GetID()).Infof("`%s` Skips app %s. Current tried=%d", p.GetID(), meta.DataID, meta.Tried)
				break
			}

			var app *customer.Application
			err := utils.RetryOperation(func() error {
				var err error
				app, err = p.WorkerAdapter.FetchInformationOfProfile(meta.DataID, true)
				return err
			}, 3, 1)
			if err == nil {
				logger.Root.Infof("`%s` finishes processing app %s. Sending to processed queue", p.GetID(), meta.DataID)
				adapter.Crawled <- basecrawler.MetaCrawlingAppResult{
					Request: meta,
					App:     app,
				}
				logger.Root.Infof("Sent %s to processed queue", meta.DataID)
			}
			if err != nil || app == nil {
				logger.Root.Errorf("`%s` Coud not get the information of profile: %s %v", p.GetID(), meta.DataID, err)
				if strings.Contains(err.Error(), "NetworkError when attempting to fetch resource") || strings.Contains(err.Error(), "connection") || strings.Contains(err.Error(), "invalid session id") {
					nedToStopBecauseOfError = true
					p.SetNeedToBeReplaced(true)
				}
				if err == ErrInvalidSession {
					p.SetNeedToBeReplaced(true)
				}
				logger.Root.With("crawlerID", p.GetID()).Infof("Send application profile %s back to the queue", meta.DataID)
				meta.Tried++
				adapter.AddCrawlingRequestBackToTheQueue(meta)
			}
		default:
			break
		}
	}
	logger.Root.Infof("`%s` exit start crawling...", p.GetID)
}

func (p *Crawler) setTheStartDayToCrawl(productOptions string) error {
	defer func() {
		(*p.Browser.DriverInfo.Driver).ExecuteScript("document.title = '"+p.GetID()+"'", []interface{}{})
	}()

	logger.Root.Infof("Browser `%s` sets the starting date & product options", p.GetID())

	today := utils.GetCurrentTime()
	lastTimeRunningCrawler := p.ClientAdapter.GetHostApplication().GetWorkerConfig().Crawler.LastCrawlingDate
	logger.Root.Infof("Last crawling time: %s", lastTimeRunningCrawler.Format(time.RFC1123))
	// if today.Day() != lastTimeRunningCrawler.Day() || p.transactionID == "" {

	p.ClientAdapter.GetHostApplication().GetConfig().CurrentCrawlingPage = 1
	lastTimeRunningCrawlerUnix := lastTimeRunningCrawler.Unix()
	N := p.ClientAdapter.GetHostApplication().GetWorkerConfig().Crawler.LatestDateInThePast
	if N < 0 {
		N = 0
	}

	lastNDaysUnix := today.Add(time.Duration(-N*24-3) * time.Hour).Unix()
	if lastNDaysUnix > lastTimeRunningCrawlerUnix && N > 0 {
		logger.Root.Infof("use the last time from last %d days", N)
		today = time.Unix(lastNDaysUnix, 0)
	} else {
		logger.Root.Infof("use the last crawling time")
		today = time.Unix(lastTimeRunningCrawlerUnix-4*60*60, 0) // shift left 2 hour to make sure we get everthing
	}
	logger.Root.Infof("Trying to get data from date %v\n", today)

	err := utils.RetryOperation(func() error {
		var err error
		err = p.GoToReportPage()
		if err != nil {
			logger.Root.Errorf("`%s` Error when going to the report page", p.GetID())
			return err
		}
		(*p.Browser.DriverInfo.Driver).ExecuteScript("document.title = '"+p.GetID()+"'", []interface{}{})

		// crawler.applyFilter("Last 30 Days")
		// crawler.applyFilter("Today")
		// err = p.applyFilter(today.Format("02/1/2006 15:04"))
		err = p.applyFilter(today.Format("Jan 2, 2006 3:04:05 PM")) // Dec 1, 2021 5:08:00 PM
		if err != nil {
			logger.Root.Errorf("`%s` Error when setting the time options", p.GetID())
			return err
		}

		if strings.Trim(productOptions, " \n\t") != "" {
			err = p.applyFilterForProduct(productOptions)
			if err != nil {
				logger.Root.Errorf("`%s` Error when setting the product options", p.GetID())
				return err
			}
		}

		if err := p.gotoListUI(); err != nil {
			logger.Root.Errorf("`%s` Error when clicking the list button", p.GetID())
			return err
		} else {
			logger.Root.Infof("`%s` Go to the list UI ok!", p.GetID())
		}

		d := *p.Browser.DriverInfo.Driver
		elm, err := d.FindElement(selenium.ByXPATH, "//*[@sortfield='@@pxDay(.pxCreateDateTime)']")
		if err == nil {
			logger.Root.Infof("Clicking on the header ENTERED ON")
			elm.MoveTo(0, 0)
			elm.Click()
			time.Sleep(2000 * time.Millisecond)
			basecrawler.WaitUntilElementDisplay(d, "//*[@id='sort']", 10, 1, 1, 2)
		} else {
			logger.Root.Errorf("Could not click on the header `Entered On`")
		}

		return nil
	}, 5, 5*time.Second)

	if err != nil {
		return err
	}
	// } else {
	// 	logger.Root.Infof("No need to set the date and product options")
	// }

	return nil
}

// switch from the summary UI to list UI
func (p *Crawler) gotoListUI() error {
	d := *(p.Browser.DriverInfo.Driver)
	defer func() {
		(*p.Browser.DriverInfo.Driver).ExecuteScript("document.title = '"+p.GetID()+"'", []interface{}{})
	}()

	err := basecrawler.WaitUntilElementDisplay(d, "//tr[@id='GrandTotal']/td[2]", 120, 1, 10, 3)
	if err != nil {
		logger.Root.Errorf("Could not find the total number of applications")

		return err
	}

	menuActivator, err := d.FindElement(selenium.ByXPATH, "//tr[@id='GrandTotal']/td[2]")
	// if there is a button that activates the menu containing the item "List"
	if err == nil {
		menuActivator.MoveTo(0, 0)
		menuActivator.Click()
		time.Sleep(300 * time.Millisecond)
		logger.Root.Infof("Clicked on the the number of total number of application")
	}

	time.Sleep(3 * time.Second)
	err = basecrawler.WaitUntilElementDisplay(d, "//*[@id='PEGA_GRID0']", 240, 1, 10, 3)

	if err != nil {
		logger.Root.Errorf("Could not see the list. Error: %v", err)
		return err
	}

	return nil
}

func (p *Crawler) applyFilter(timeOption string) error {
	// //*[contains(@onclick, 'showSingleFilterSection')]
	d := *(p.Browser.DriverInfo.Driver)
	elms, err := d.FindElements(selenium.ByXPATH, "//*[contains(@onclick, 'showSingleFilterSection')]")
	if err != nil || len(elms) == 0 {
		logger.Root.Errorf("Could not found the button to show options. Err: %v", err)
		return err
	}

	elms[0].MoveTo(0, 0)
	elms[0].Click()

	time.Sleep(3 * time.Second)
	// wait the dialog appear
	err = basecrawler.WaitUntilElementDisplay(d, "//*[contains(@onclick, 'pickerpopup.pickValuePopup')]|//*[contains(@data-click, 'pzPopUp_PickValuesContentFA')]", 120, 1, 10, 3)

	if err != nil {
		logger.Root.Errorf("Could not find the SelectValue button. %v", err)
		return err
	}

	valueBoxElm, err := d.FindElement(selenium.ByXPATH, "//*[contains(@name, '$PpyReportContentPage$ppyReportDefinition$ppyUI$ppyBody$ppyUIFilters$ppyFilter$l2$ppyFilterValue')]")
	if err != nil {
		logger.Root.Errorf("Could not find the value box. %v", err)
		return ErrTimeout
	}
	valueBoxElm.Clear()
	valueBoxElm.SendKeys(timeOption)

	// apply the change

	elm, err := d.FindElement(selenium.ByXPATH, "//button[contains(@name, 'pzReportConfigApplyCancel')]")
	if err != nil {
		logger.Root.Errorf("Error when click button Apply. %v", err)
		return err
	}

	elm.MoveTo(0, 0)
	elm.Click()

	time.Sleep(1 * time.Second)
	// wait until the Apply Dialog disapear
	basecrawler.WaitUntilElementDisappear(d, "//select[@id='pyLabel']", 120, 1, 10, 3)

	// //button[contains(@name, 'pzReportConfigApplyCancel')]

	// a wanring modal can appear
	// "You are about to close an open work item which has changes that have not been saved. "
	// try to close onclick="doModalAction('', event);
	// err = d.WaitWithTimeoutAndInterval(func(wd selenium.WebDriver) (bool, error) {
	// 	errElement, err := wd.FindElement(selenium.ByXPATH, "//*[contains(@class, 'pzbtn-mid')]")
	// 	if err != nil {
	// 		return false, err
	// 	}

	// 	displayed, _ := errElement.IsDisplayed()
	// 	return displayed, nil
	// }, 120*time.Second, 1*time.Second)

	return nil
}

func (p *Crawler) applyFilterForProduct(productOptions string) error {
	// //*[contains(@onclick, 'showSingleFilterSection')]
	d := *(p.Browser.DriverInfo.Driver)
	err := basecrawler.WaitUntilElementDisplay(d, "//*[contains(@data-click, 'convertToList')]", 120, 1, 10, 3)
	if err != nil {
		logger.Root.Errorf("Could not find the List button")
		return err
	}
	elms, err := d.FindElements(selenium.ByXPATH, "//*[contains(@onclick, 'showSingleFilterSection')]")
	if err != nil || len(elms) == 0 {
		logger.Root.Errorf("Could not found the button to show options. Err: %v", err)
		return err
	}

	elms[2].MoveTo(0, 0)
	elms[2].Click()

	time.Sleep(1 * time.Second)
	// wait the dialog appear
	err = basecrawler.WaitUntilElementDisplay(d, "//*[contains(@onclick, 'pickerpopup.pickValuePopup')]", 120, 1, 10, 3)

	if err != nil {
		logger.Root.Errorf("Could not find the SelectValue button. %v", err)
		return err
	}

	valueBoxElm, err := d.FindElement(selenium.ByXPATH, "//*[contains(@name, '$PpyReportContentPage$ppyReportDefinition$ppyUI$ppyBody$ppyUIFilters$ppyFilter$l4$ppyFilterValue')]")
	if err != nil {
		logger.Root.Errorf("Could not find the value box. %v", err)
		return ErrTimeout
	}
	valueBoxElm.Clear()
	valueBoxElm.SendKeys(productOptions)

	// apply the change

	elm, err := d.FindElement(selenium.ByXPATH, "//button[contains(@name, 'pzReportConfigApplyCancel')]")
	if err != nil {
		logger.Root.Errorf("Error when click button Apply. %v", err)
		return err
	}

	elm.MoveTo(0, 0)
	elm.Click()

	time.Sleep(1 * time.Second)
	// wait until the Apply Dialog disapear
	basecrawler.WaitUntilElementDisappear(d, "//select[@id='pyLabel']", 120, 1, 10, 3)

	// //button[contains(@name, 'pzReportConfigApplyCancel')]

	// a wanring modal can appear
	// "You are about to close an open work item which has changes that have not been saved. "
	// try to close onclick="doModalAction('', event);
	// err = d.WaitWithTimeoutAndInterval(func(wd selenium.WebDriver) (bool, error) {
	// 	errElement, err := wd.FindElement(selenium.ByXPATH, "//*[contains(@class, 'pzbtn-mid')]")
	// 	if err != nil {
	// 		return false, err
	// 	}

	// 	displayed, _ := errElement.IsDisplayed()
	// 	return displayed, nil
	// }, 120*time.Second, 1*time.Second)

	return nil
}

func (p *Crawler) fetchPageWithBrowser(page int, pageSize int) (string, bool, error) {
	d := (*p.Browser.DriverInfo.Driver)

	basecrawler.WaitUntilElementDisplay(d, "//*[@id='pyGridActivePage']", 5, 1, 1, 3)

	elm, err := d.FindElement(selenium.ByXPATH, "//*[@id='pyGridActivePage']")
	if err != nil {
		time.Sleep(5 * time.Second)
		src, _ := d.PageSource()
		if !strings.Contains(src, "Entered On") && !strings.Contains(src, "Displaying ") {
			if strings.Contains(src, "You are about to close an open work") {
				logger.Root.Infof("The page doesn't show content (close open work)")
				return "", false, ErrInvalidSession
			}
			logger.Root.Errorf("Error when finding the current active page. Err: %v", err)
			utils.WriteContentToPage([]byte(src), "no-active-page-", "empty-work-ids")
			return "", false, err
		}

		return src, strings.Contains(src, "Next Page"), nil
	}
	text, _ := elm.Text()
	text = strings.Trim(text, " \n\t")
	label, _ := elm.GetAttribute("aria-label")
	logger.Root.Infof("Label of current page: %s", label)

	if v, err := strconv.Atoi(text); err == nil && v == page-1 {
		if elm, err = d.FindElement(selenium.ByXPATH, "//*[@title='Next Page']"); err == nil {
			// elm.Click()
		} else {
			logger.Root.Errorf("Error when finding the current page. Switch to ajax verion. Err: %v", err)
			return p.fetchPage(page, pageSize)
		}
	} else {

		elm, err = d.FindElement(selenium.ByXPATH, fmt.Sprintf("//*[contains(@aria-label, 'Page %d')]", page))
		if err != nil {

			logger.Root.Errorf("Error when finding the previous page. Switch to ajax verion. Err: %v", err)
			return p.fetchPage(page, pageSize)

		}
	}

	// elm, err := d.FindElement(selenium.ByXPATH, "//*[@id='pyGridActivePage']")
	// if err != nil {
	// 	logger.Root.Errorf("Error when finding the current active page. Switch to ajax verion. Err: %v", err)
	// 	return p.fetchPage(page, pageSize)
	// }

	// _, err = d.ExecuteScript(fmt.Sprintf(`var elm =document.getElementById('pyGridActivePage'); elm.setAttribute('href', '#'); elm.addEventListener('click', function(){ Grids.getActiveGrid(event).gridPaginator(event,'%d'); return false; }); `, page), nil)
	// if err != nil {
	// 	logger.Root.Errorf("Error when execute ajax to set the page %v", err)
	// }

	elm.MoveTo(0, 0)
	elm.Click()
	time.Sleep(1000 * time.Millisecond)

	err = basecrawler.WaitUntilElementDisplay(d, fmt.Sprintf("//*[contains(@aria-label, 'Current Page. Page %d')]", page), 240, 1, 10, 3)
	// err = waitUntilElementDisplay(d, "//*[@id='PEGA_GRID0']", 240, 1, 10, 3)
	time.Sleep(500 * time.Millisecond)

	if err != nil {
		logger.Root.Errorf("Could not see the list. Error: %v. Switch to the ajax version to get the page", err)
		return p.fetchPage(page, pageSize)
	}

	pageContent, err := d.PageSource()

	if err != nil {
		logger.Root.Errorf("Could not get page %d, error: %v", page, err)
	} else {
		// logger.Root.Infof("Result of page: %d `%s`", page, result)
		logger.Root.Infof("Fetch page %d OK", page)
	}
	logger.Root.Infof("finish getting page %d with pagesize %d", page, pageSize)

	hasNextPage := strings.Contains(pageContent, "Next Page")
	logger.Root.Infof("Current page: %d Has next page: %v", page, hasNextPage)
	return pageContent, hasNextPage, nil

}

func (p *Crawler) fetchPage(page int, pageSize int) (string, bool, error) {
	url := BaseURL + `!TABTHREAD1?pyActivity=ReloadSection&pzTransactionId=@transactionID@&pzFromFrame=pyReportContentPage&pzPrimaryPageName=pyReportContentPage&StreamName=pzRRListBodyRK&RenderSingle=EXPANDEDSubSectionpzRRListBodyRKB~pzLayout_1&StreamClass=Rule-HTML-Section&bClientValidation=true&PreActivity=pzdoGridAction&ActivityParams=gridAction%3DPAGINATE%26gridActivity%3DpzGridSortPaginate%26pyPaginateActivity%3D%26PageListProperty%3DpyReportContentPage.pxResults%26ClassName%3DFECredit-Base-Loan-Work-Loan%26RDName%3DNumberOfAppsByStage%26RDAppliesToClass%3DFECredit-Base-Loan-Work-Loan%26pgRepContPage%3DpyReportContentPage%26pyReportPageName%3DpyReportContentPage.pyReportDefinition%26RDParamsList%3D%26gridLayoutID%3DSubSectionpzRRListBodyRKB%26BaseReference%3D%26startIndex%3D@startIndex@%26currentPageIndex%3D@page@%26pyPageMode%3DNumeric%26pyPageSize%3D@pageSize@%26sortProperty%3D%40%40pxDay(.pxCreateDateTime)%26sortType%3DASC%26isReportDef%3Dtrue%26prevStartIndex%3D@preStartIndex@&BaseReference=&ReadOnly=0&FieldError=&FormError=NONE&pyCustomError=DisplayErrors&Increment=true&inStandardsMode=true&AJAXTrackID=1&pzHarnessID=@harnessID@`

	p.getHarnessID("HID64F81A0B39D81B83371A71E55A666441")
	url = strings.ReplaceAll(url, "@id@", p.loginURLID)
	url = strings.ReplaceAll(url, "@transactionID@", p.transactionID)
	url = strings.ReplaceAll(url, "@page@", fmt.Sprint(page))
	url = strings.ReplaceAll(url, "@pageSize@", fmt.Sprint(pageSize))
	url = strings.ReplaceAll(url, "@startIndex@", fmt.Sprint((page-1)*pageSize+1))
	if (page-2)*pageSize+1 >= 0 {
		url = strings.ReplaceAll(url, "@preStartIndex@", fmt.Sprint((page-2)*pageSize+1))
	} else {
		url = strings.ReplaceAll(url, "@preStartIndex@", "")
	}
	url = strings.ReplaceAll(url, "@harnessID@", p.harnessID)

	logger.Root.Infof("Get page %d , pagesize %d with url `%s`", page, pageSize, url)

	payload := `&pyReportContentPagePpxResults1colWidthGBL=& pyReportContentPagePpxResults1colWidthGBR=&pyReportContentPage.pxResults1colWidthCache1=&SectionIDList=GID_@timestamp@%3A&$OCompositeAPIInclude=&$ODesktopWrapperInclude=&$OEvalDOMScripts_Include=&$OFusionChart=&$OHarnessInlineScripts=&$OInteractiveChartScript=&$OLaunchFlowScriptInclude=&$OListViewIncludes=&$OMenuBar=&$OWorkformStyles=&$OdocumentInfo_bundle=&$OmenubarInclude=&$OpyWorkFormStandard=&$OpzHarnessStaticScripts=&$OpzMobileAppNotification=&$OpzTextInput=&$OxmlDocumentInclude=&$OPegaToolsCacheInclude=&$OpzAutoComplete_Variables=&$ODateTime=&$ODateTimeScript=&$OSpecCheckerScript=&$Ogridincludes=&$OpzControlMenuScripts=`
	payload = strings.ReplaceAll(payload, "@timestamp@", fmt.Sprint(time.Now().UnixNano()/1000))
	headers := map[string]interface{}{
		"credentials": "include",
		"headers": map[string]string{
			"User-Agent":       "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:84.0) Gecko/20100101 Firefox/84.0",
			"Accept":           "*/*",
			"Accept-Language":  "en-US,en;q=0.5",
			"X-Requested-With": "XMLHttpRequest",
			"Content-Type":     "application/x-www-form-urlencoded",
		},
		// "referrer": "https://bpmportal.fecredit.com.vn/prweb/PRWebLDAP2/@id@/!TABTHREAD1?pyActivity=Rule-Shortcut.pxRunShortcut&InsKey=RULE-SHORTCUT%20FECREDITSUPERVISORREPORTS%20S!ALL!NUMBEROFCURRENTAPPLICATIONSBYSTAGE%20%2320170124T034251.673%20GMT&pzHarnessID=@harnessID@",
		"body":   payload,
		"method": "POST",
		"mode":   "cors",
	}
	headerInByte, _ := json.Marshal(headers)
	headerInStr := string(headerInByte)
	headerInStr = p.formatCommonURL(headerInStr, "")
	pageContent, err := p.Browser.SendAjax(url, "POST", payload, string(headerInByte), 240, func(result string, err error) {
	})

	if err != nil {
		logger.Root.Errorf("Could not get page %d, error: %v", page, err)
	} else {
		// logger.Root.Infof("Result of page: %d `%s`", page, result)
		logger.Root.Infof("Fetch page %d OK", page)
	}
	logger.Root.Infof("finish getting page %d with pagesize %d", page, pageSize)

	return pageContent, strings.Contains(pageContent, "Next Page"), nil

}

func (p *Crawler) parsePageToGetWorkIDs(content string) []string {
	doc, err := htmlquery.Parse(strings.NewReader(content))

	if err != nil {
		logger.Root.Errorf("\n\n\nError when parsing xml: %s\n\n\n", err)
		return []string{}
	}

	trElements := htmlquery.Find(doc, "//*[@id='gridLayoutTable']//table[@id='bodyTbl_right']//tr")
	if pzHarnessIDElm := htmlquery.FindOne(doc, "//input[@id='pzHarnessID']"); pzHarnessIDElm != nil {
		newpzHarnessID := htmlquery.SelectAttr(pzHarnessIDElm, "value")
		if newpzHarnessID != "" {
			logger.Root.Infof("Updated the pzHarnessID to %s", newpzHarnessID)
			p.harnessID = newpzHarnessID
		}
	}

	workIDs := []string{}
	for i, tr := range trElements {
		tdWorkID := htmlquery.FindOne(tr, "//td[@data-attribute-name='ID']")
		if tdWorkID != nil {
			workID := strings.Trim(htmlquery.InnerText(tdWorkID), " \n\t")
			workIDs = append(workIDs, workID)
		} else {
			if i > 0 {
				logger.Root.Errorf("Could not found element `%s`", "//td[@data-attribute-name='Work ID']")
			}
		}
	}

	return workIDs
}
