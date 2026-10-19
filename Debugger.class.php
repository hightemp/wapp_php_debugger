<?php

if (defined('DA_LOADED')) {
    return 'defined DA_LOADED';
}

define('DA_LOADED', true);

define('DA_CONFIG_FILE_NAME', 'dws_debugger.json');
define('DA_CONFIG_FILE_PATH', __DIR__."/".DA_CONFIG_FILE_NAME);

// [!] Все пути должны быть без / на конце
// Константы можно определить до подключения файла, иначе берутся значения по умолчанию
defined('DA_ROOT_DIR') || define('DA_ROOT_DIR', __DIR__);
// Отладчик активен, только если есть файл конфига (включение и выключение — флаг bIsOnFlag в нём)
defined('DA_FLAG') || define('DA_FLAG', is_file(DA_CONFIG_FILE_PATH));
// HTTP-управление (?debug_msg=..., ?save_debug_message) выключено, пока не задан токен (не короче 16 символов).
// Токен передаётся в заголовке X-Debugger-Token.
// define('DA_HTTP_TOKEN', '...');

// define('DA_LOGS_MESSAGES_DIR', DA_ROOT_DIR."/logs/dws_debugger");

define('DWS_WARNING', 'warning');
define('DWS_ERROR', 'error');
define('DWS_INFO', 'info');
define('DWS_MESSAGE', 'message');
define('DWS_FUNC_DEBUG', 'functions_debug');

define('DEBUG_MSG', 'debug_msg');

class Debugger
{
    public $sConfigFilePath = DA_CONFIG_FILE_PATH;
    public $iConfigFileMod = 0666;

    public $sProjectRootPath = '';
    public $sTempPath = ''; 
    public $sMessagesFilesPath = '';
    public $bIsOnFlag = false;

    public $iPage = 0;
    public $iPageSize = 15;

    public $iArraySizeLimit = 0;
    public $iLogsSizeLimit = 0;

    public $fnOldErrorHandler = null;

    public $iRequestTimestamp;
    public $sLogFilePath;
    public $sURL = '';
    public $bIsCli = false;
    public $bIsAjax = false;
    public $sIP = '';
    public $sServerIP = '';
    public $sServerName = '';
    public $sUser = '';
    public $sSysUser = '';
    public $sLang = '';
    public $bFinished = false;
    public $iCode = null;

    public $iStartTime = 0;
    public $iStopTime = 0;
    public $iTotalTime = 0;

    public $aFoundFields = [];
    public $aNotFoundFields = [];

    public $aTimers = [];
    public $aNamedTimers = [];
    public $iTimerCounter = 0;

    public static function fnGetInstance()
    {
        static $oInstance;
        return $oInstance ? $oInstance : $oInstance = new self();
    }

    function fnGetDebugParam()
    {
        $aArr = array_filter($_GET, function($k) { return strpos($k, DEBUG_MSG) !== false;}, ARRAY_FILTER_USE_KEY);
        if (count($aArr)) {
            return array_pop($aArr);
        }
    }

    function fnSendResponse($mVar)
    {
        header("Content-Type: application/json");
        die(json_encode($mVar, JSON_UNESCAPED_UNICODE));
    }

    function fnGetProtocol()
    {
        return (
            isset($_SERVER['HTTPS']) && $_SERVER['HTTPS'] === 'on' 
            ? "https" 
            : "http"
        );
    }

    function fnGetCurrentURL()
    {
        if (!isset($_SERVER['HTTP_HOST'])) {
            return '';
        }

        $sURL = $_SERVER['HTTP_HOST'].(isset($_SERVER['REQUEST_URI']) ? $_SERVER['REQUEST_URI'] : '');

        return $this->fnGetProtocol() . "://" . $sURL;
    }

    function fnToHumanSize($iSize)
    {
        $aUnits = ['B', 'K', 'M', 'G', 'T'];
        $iUnit = 0;

        while ($iSize >= 1024 && $iUnit < count($aUnits) - 1) {
            $iSize /= 1024;
            $iUnit++;
        }

        return round($iSize, 1).$aUnits[$iUnit];
    }

    // Формат как у строки `df -h`: [Filesystem, Size, Used, Avail, Use%, Mounted on]
    function fnGetCurrentSpace()
    {
        $iTotal = @disk_total_space($this->sTempPath);
        $iFree = @disk_free_space($this->sTempPath);

        if (!$iTotal || $iFree === false) {
            return [];
        }

        $iUsed = $iTotal - $iFree;

        return [
            "",
            $this->fnToHumanSize($iTotal),
            $this->fnToHumanSize($iUsed),
            $this->fnToHumanSize($iFree),
            round($iUsed / $iTotal * 100)."%",
            $this->sTempPath,
        ];
    }

    // Размер файлов в папке в килобайтах (как `du -s`), чтобы iLogsSizeLimit в старых конфигах сохранил смысл
    function fnGetDirSize($sPath)
    {
        $iSize = 0;

        foreach ((array) glob($sPath."/*") as $sFilePath) {
            if (is_file($sFilePath)) {
                $iSize += (int) @filesize($sFilePath);
            }
        }

        return (int) ceil($iSize / 1024);
    }

    function fnIsSafeMessagesFilesPath()
    {
        return !empty($this->sMessagesFilesPath) && count(explode('/', $this->sMessagesFilesPath)) >= 3;
    }

    // Удаляет содержимое папки без exec; по символическим ссылкам не переходит
    function fnRemoveDirContents($sPath)
    {
        if (!is_dir($sPath)) {
            return;
        }

        $oIterator = new RecursiveIteratorIterator(
            new RecursiveDirectoryIterator($sPath, FilesystemIterator::SKIP_DOTS),
            RecursiveIteratorIterator::CHILD_FIRST
        );

        foreach ($oIterator as $oFile) {
            if ($oFile->isDir() && !$oFile->isLink()) {
                @rmdir($oFile->getPathname());
            } else {
                @unlink($oFile->getPathname());
            }
        }
    }

    function fnGetCurrentUser()
    {
        if (!function_exists('posix_getpwuid') || !function_exists('posix_geteuid')) {
            return '';
        }

        $processUser = posix_getpwuid(posix_geteuid());
        return @$processUser['name'];
    }

    function fnGetServerIP()
    {
        return @$_SERVER['SERVER_ADDR'];
    }

    function fnGetServerName()
    {
        return @$_SERVER['SERVER_NAME'];
    }

    function fnGetIP()
    {
        return @$_SERVER['HTTP_X_FORWARDED_FOR'] ? $_SERVER['HTTP_X_FORWARDED_FOR'] : @$_SERVER['REMOTE_ADDR'];
    }

    function fnGetSysUser()
    {
        return isset($_SESSION['USER_INFO']['login']) ? $_SESSION['USER_INFO']['login'] : '';
    }

    public static $aConfigKeys = [
        "sProjectRootPath",
        "sTempPath",
        "sMessagesFilesPath",
        "bIsOnFlag",
        "iPageSize",
        "iArraySizeLimit",
        "iLogsSizeLimit",
        "bLogSessionAndCookies"
    ];

    // Ключи, которые можно менять через HTTP. Пути меняются только правкой конфига на сервере
    public static $aRequestConfigKeys = [
        "bIsOnFlag",
        "iPageSize",
        "iArraySizeLimit",
        "iLogsSizeLimit"
    ];

    // Значения с такими ключами не попадают в лог
    public $sSecretKeysPattern = '/(pass(wd|word)?$|^pwd$|secret|token|api_?key|app_?key|private_?key|access_?key|authori[sz]ation|php_auth_pw|cookie|dsn|csrf)/i';
    public $bLogSessionAndCookies = false;

    function fnIsHttpApiAllowed()
    {
        if (!defined('DA_HTTP_TOKEN') || !is_string(DA_HTTP_TOKEN) || strlen(DA_HTTP_TOKEN) < 16) {
            return false;
        }

        $sToken = isset($_SERVER['HTTP_X_DEBUGGER_TOKEN']) ? (string) $_SERVER['HTTP_X_DEBUGGER_TOKEN'] : '';

        return hash_equals(DA_HTTP_TOKEN, $sToken);
    }

    function fnFilterRequestConfig($aParams)
    {
        $aResult = [];

        foreach (self::$aRequestConfigKeys as $sKey) {
            if (isset($aParams[$sKey])) {
                $aResult[$sKey] = $sKey[0] == 'b'
                    ? filter_var($aParams[$sKey], FILTER_VALIDATE_BOOLEAN)
                    : (int) $aParams[$sKey];
            }
        }

        return $aResult;
    }

    function fnMaskSecrets($mVars, $iDepth=0)
    {
        if (!is_array($mVars) || $iDepth > 32) {
            return $mVars;
        }

        foreach ($mVars as $mKey => $mValue) {
            if (is_string($mKey) && preg_match($this->sSecretKeysPattern, str_replace('-', '_', $mKey))) {
                $mVars[$mKey] = '***';
            } else if (is_array($mValue)) {
                $mVars[$mKey] = $this->fnMaskSecrets($mValue, $iDepth + 1);
            }
        }

        return $mVars;
    }

    function fnSaveConfig($aParams=[])
    {
        foreach (self::$aConfigKeys as $sKey) {
            $aConfig[$sKey] = isset($aParams[$sKey]) ? $aParams[$sKey] : $this->$sKey;
        }

        file_put_contents($this->sConfigFilePath, json_encode($aConfig, JSON_PRETTY_PRINT | JSON_UNESCAPED_UNICODE));
        chmod($this->sConfigFilePath, $this->iConfigFileMod);

        $this->fnLoadConfig();
        $this->fnMakeDirsIfNotExist();
    }

    function fnLoadConfig()
    {
        if (is_file($this->sConfigFilePath)) {
            $aConfig = json_decode(file_get_contents($this->sConfigFilePath), true);

            foreach (self::$aConfigKeys as $sKey) {
                if (isset($aConfig[$sKey])) {
                    $this->$sKey = $aConfig[$sKey];
                }
            }
        }
    }

    // Удаляет самые старые файлы логов, пока суммарный размер больше iLogsSizeLimit (в килобайтах)
    function fnClearLargeLogs()
    {
        if (!$this->fnIsSafeMessagesFilesPath()) {
            return;
        }

        $iSize = $this->fnGetDirSize($this->sMessagesFilesPath);

        if ($this->iLogsSizeLimit >= $iSize) {
            return;
        }

        $aFiles = [];
        foreach ((array) glob($this->sMessagesFilesPath."/*") as $sFilePath) {
            if (is_file($sFilePath)) {
                $aFiles[$sFilePath] = (int) @filemtime($sFilePath);
            }
        }
        asort($aFiles);

        $iSizeBytes = $iSize * 1024;
        foreach ($aFiles as $sFilePath => $iMTime) {
            if ($iSizeBytes <= $this->iLogsSizeLimit * 1024) {
                break;
            }
            $iFileSize = (int) @filesize($sFilePath);
            if (@unlink($sFilePath)) {
                $iSizeBytes -= $iFileSize;
            }
        }
    }

    function __construct()
    {
        $this->fnLoadConfig();

        if ($this->bIsOnFlag) {
            $this->fnOldErrorHandler = set_error_handler([$this, "fnDebugErrorHandler"]);
            $this->iStartTime = $this->milliseconds();

            if ($this->iLogsSizeLimit) {
                $this->fnClearLargeLogs();
            }
        }

        // Без токена HTTP-команды не обрабатываются: запрос идёт в приложение как обычно
        if (!$this->fnIsHttpApiAllowed()) {
            return;
        }

        $sDebugMsg = $this->fnGetDebugParam();

        if (isset($_GET['save_debug_message'])) {
            $aData = isset($_POST['sData']) ? json_decode($_POST['sData'], true) : null;
            if (!is_array($aData) || !isset($aData['sType'], $aData['sMessage'])) {
                $this->fnSendResponse("ERR");
            }
            $this->fnDebugLog(
                (string) $aData['sType'],
                (string) $aData['sMessage'],
                isset($aData['aVars']) && is_array($aData['aVars']) ? $aData['aVars'] : [],
                isset($aData['iRequestTimestamp']) ? (int) $aData['iRequestTimestamp'] : 0,
                "",
                isset($aData['sLang']) ? (string) $aData['sLang'] : "js"
            );
            $this->fnSendResponse("OK");
        }

        if (is_string($sDebugMsg)) {
            if (isset($_GET['page'])) $this->iPage = ((int) $_GET['page'])-1;
            if (isset($_GET['page_size'])) $this->iPageSize = (int) $_GET['page_size'];
            
            // $this->iPageSize = 15;
    
            if ($sDebugMsg=='status') {
                $this->fnMakeDirsIfNotExist();

                $aResult = [ 
                    "bIsOn" => $this->bIsOnFlag,
                    "sDebuggerPath" => __DIR__,
                    "aDiskSpace" => $this->fnGetCurrentSpace(),
                    "sProjectRootPath" => $this->sProjectRootPath,
                    "bProjectRootPathExists" => is_dir($this->sProjectRootPath),
                    "sTempPath" => $this->sTempPath,
                    "bTempPathExists" => is_dir($this->sTempPath),
                    "sConfigFilePath" => $this->sConfigFilePath,
                    "sMessagesFilesPath" => $this->sMessagesFilesPath,
                    "bMessagesFilesPathExists" => is_dir($this->sMessagesFilesPath),
                    "iCRC32" => crc32(file_get_contents(__FILE__)),
                ];
                $this->fnSendResponse($aResult);
            }

            if ($sDebugMsg=='set_config') {
                $this->fnSaveConfig($this->fnFilterRequestConfig($_POST));
                $this->fnSendResponse("OK");
            }

            if ($sDebugMsg=='start') {
                $this->bIsOnFlag = true;
                $this->fnSaveConfig(array_merge($this->fnFilterRequestConfig($_POST), ["bIsOnFlag" => true]));
                $this->fnSendResponse("OK");
            }
    
            if ($sDebugMsg=='stop') {
                $this->bIsOnFlag = false;
                $this->fnSaveConfig();
                $this->fnSendResponse("OK");
            }
    
            if ($sDebugMsg=='clear') {
                if (!$this->fnIsSafeMessagesFilesPath()) {
                    $this->fnSendResponse("ERR");//: path '{$this->sMessagesFilesPath}' is wrong");
                }
                $sMask = "{$this->sMessagesFilesPath}/*";
                $this->fnRemoveDirContents($this->sMessagesFilesPath);

                $iFilesCount = count((array) glob($sMask));
                if ($iFilesCount) {
                    $this->fnSendResponse("ERR");// : dir is not empty, {$iFilesCount} files found");
                }

                $this->fnSendResponse("OK");
            }
    
            if ($sDebugMsg=='info') {
                $aFileInfoAssocArray = [];
                foreach (glob($this->sMessagesFilesPath."/*") as $sFilePath) {
                    $sFileName = @basename($sFilePath);
                    $aFileInfo = $this->fnGetFileMessages($sFileName, true);
                    $aFileInfoAssocArray[$sFileName] = [
                        "sFile" => $sFileName,
                        "iFileSize" => @filesize($sFilePath),
                        "iPages" => ceil($aFileInfo["iCount"]/$this->iPageSize),
                    ];
                    $aFileInfoAssocArray[$sFileName] = array_merge($aFileInfoAssocArray[$sFileName], $aFileInfo);
                }

                $this->fnSendResponse($aFileInfoAssocArray);
            }
    
            if ($sDebugMsg=='get_file') {
                $sRootPath = realpath($this->sProjectRootPath);
                $sFileFullPath = isset($_GET["file"]) ? realpath($this->sProjectRootPath."/".$_GET["file"]) : false;

                if (
                    !$sRootPath
                    || !$sFileFullPath
                    || strpos($sFileFullPath, rtrim($sRootPath, DIRECTORY_SEPARATOR).DIRECTORY_SEPARATOR) !== 0
                    || !is_file($sFileFullPath)
                ) {
                    $this->fnSendResponse("ERR");
                }

                $sContent = file_get_contents($sFileFullPath);
                $this->fnSendResponse($sContent);
            }
    
            if (!$sDebugMsg) {
                $aFiles = [];
                foreach (glob($this->sMessagesFilesPath."/*") as $sFilePath) {
                    $aFiles[] = basename($sFilePath);
                }
                $this->fnSendResponse($aFiles);
            }
    
            $sMessagesFilePath = $this->sMessagesFilesPath."/".$sDebugMsg;
            if (basename($sDebugMsg) === $sDebugMsg && is_file($sMessagesFilePath)) {
                if (isset($_GET['download'])) {
                    $this->fnSendResponse(
                        json_encode(
                            $this->fnGetFileMessages(
                                $sDebugMsg,
                                false,
                                true
                            ), 
                            JSON_PRETTY_PRINT
                            | JSON_UNESCAPED_UNICODE
                        )
                    );
                }
                $this->fnSendResponse($this->fnGetFileMessages($sDebugMsg));
            }
    
            $this->fnSendResponse("error");
        }
    
    }

    function milliseconds() 
    {
        $mt = explode(' ', microtime());
        return ((int)$mt[1]) * 1000 + ((int)round($mt[0] * 1000));
    }

    function fnDebugErrorHandler($iErrno, $sErrstr, $sErrfile, $iErrline)
    {    
        $sMessage = '';

        if (in_array($iErrno, [E_USER_ERROR/*, E_DEPRECATED, E_USER_DEPRECATED*/])) {
            $sMessage = "[ERROR][$iErrno] $sErrstr: $sErrfile:$iErrline";
            $this->fnDebugLog(DWS_ERROR, $sMessage, func_get_args());
        }
    }

    function fnDebugShutdown()
    {
        if (!(PHP_SAPI == 'cli' || !isset($_SERVER['HTTP_USER_AGENT'])) && function_exists('fastcgi_finish_request')) {
            fastcgi_finish_request();
        }

        $this->fnStopAllTimers();
        $this->iStopTime = $this->milliseconds();

        // return;
        $this->bFinished = true;
        $this->iCode = http_response_code();
        
        $err = error_get_last();
        if (in_array(@$err['type'], [E_ERROR, E_PARSE, E_CORE_ERROR, E_COMPILE_ERROR, E_USER_ERROR, E_RECOVERABLE_ERROR])) {
            $this->fnDebugLog(DWS_ERROR, "[ERROR] {$err['message']}", $err);
        }

        // if (defined('DWS_FUNC_DEBUG')) fnDebugLog(DWS_FUNC_DEBUG, "", func_get_args());
        $this->fnDebugLog(
            DWS_MESSAGE, 
            "Поля (Найдено: ".count($this->aFoundFields).", Не найдено: ".count($this->aNotFoundFields).")", 
            [ 
                'Найденные поля' => $this->aFoundFields, 
                'Не найденные поля' => $this->aNotFoundFields  
            ]
        );
    }

    function fnFindTimeDiff(&$aMessages)
    {
        for ($i = 1; $i < count($aMessages); $i++) {
            $aMessages[$i]['iTimestampDiff'] = $aMessages[$i]['iTimestamp'] - $aMessages[$i-1]['iTimestamp'];
        }
    }

    function fnSliceMessages(&$aMessages)
    {
        $iStartPos = $this->iPage*$this->iPageSize;
        $iStartPosWithPrev = $iStartPos ? $iStartPos - 1 : 0;
        $aMessages = array_slice($aMessages, $iStartPosWithPrev, $this->iPageSize);

        $this->fnFindTimeDiff($aMessages);

        if ($iStartPosWithPrev) {
            array_shift($aMessages);
        }
    }

    function fnGetFileMessages($sFile, $bGetInfo=false, $bGetAll=false) 
    {
        if (!$this->bIsOnFlag) return;

        $sFilePath = $this->sMessagesFilesPath."/".$sFile;

        $aSplited = file($sFilePath);

        $aFileInfoData = json_decode(array_shift($aSplited), true);

        $aSplited = array_filter(
            $aSplited,
            function($v) {
                return $v;
            }
        );

        $iErrorCount = 0;
        foreach ($aSplited as &$ref_mString) {
            $ref_mString = json_decode($ref_mString, true);
            if ($ref_mString["sType"]=="error") 
                $iErrorCount++;
        }

        if (!$_GET['filter'] || $bGetAll) {
            if ($bGetInfo) {
                return $this->fnPrepareFileInfo($aFileInfoData, $aSplited, $iErrorCount);
            }

            if (!$bGetAll) {
                $this->fnSliceMessages($aSplited);
            }

            // foreach ($aSplited as &$ref_mString) {
            //     $ref_mString = json_decode($ref_mString, true);
            // }
        } else {
            $bFilterError = strpos($_GET['filter'], "type:error") !== false;
            $_GET['filter'] = preg_replace("/type:error/i", "", $_GET['filter']);
            // TODO: Сделать видимыми пробелы
            // $_GET['filter'] = trim($_GET['filter']);
            // die(var_export([$_GET['filter'], $bFilterError], 1));

            $aSplited = array_filter(
                $aSplited,
                function($aFilterItem) use ($bFilterError) {
                    $bResult = true;

                    if ($bFilterError) {
                        $bResult = $bResult && $aFilterItem["sType"]=="error";
                    }

                    if ($_GET['filter']) {
                        $bResult = $bResult && strpos($aFilterItem["sMessage"], $_GET['filter'])!==false;
                    }

                    return $bResult;
                }
            );

            if ($bGetInfo) {
                return $this->fnPrepareFileInfo($aFileInfoData, $aSplited, $iErrorCount);
            }

            $this->fnSliceMessages($aSplited);
        }

        return $aSplited;
    }

    function fnIsCLI()
    {
        return php_sapi_name() == "cli";
    }

    function fnIsAjax() {
        return isset($_SERVER['HTTP_X_REQUESTED_WITH']) && $_SERVER['HTTP_X_REQUESTED_WITH'] === 'XMLHttpRequest';
    }

    function fnPrintHeaderJSON($bUpdateData=false)
    {
        return $this->fnPrintJSONLineWithLength($this->fnPrepareHeaderData($bUpdateData));
    }

    function fnPrintJSONLineWithLength($aData, $iMaxLength=1000)
    {
        $iFlags = JSON_UNESCAPED_UNICODE | (defined('JSON_INVALID_UTF8_SUBSTITUTE') ? JSON_INVALID_UTF8_SUBSTITUTE : 0);
        $sString = json_encode($aData, $iFlags);

        // Заголовок перезаписывается на месте, поэтому он не должен выходить за $iMaxLength:
        // иначе затрутся первые сообщения. Сначала укорачиваем длинные поля, потом оставляем только служебные
        if ($sString === false || strlen($sString) > $iMaxLength) {
            $aData['aArgv'] = [];
            $aData['sURL'] = substr((string) $aData['sURL'], 0, 300);
            $sString = json_encode($aData, $iFlags);
        }

        if ($sString === false || strlen($sString) > $iMaxLength) {
            $sString = json_encode(array_intersect_key($aData, array_flip([
                "bIsCli", "bIsAjax", "iRequestTimestamp", "bFinished", "iCode", "iStartTime", "iStopTime", "iTotalTime"
            ])));
        }

        return $sString.str_repeat(" ", max(0, $iMaxLength - strlen($sString)));
    }

    function fnUpdateHeaderData()
    {
        $this->sURL = $this->fnGetCurrentURL();
        $this->bIsCli = $this->fnIsCLI();
        $this->bIsAjax = $this->fnIsAjax();
        $this->sIP = $this->fnGetIP();
        $this->sServerIP = $this->fnGetServerIP();
        $this->sServerName = $this->fnGetServerName();
        $this->sUser = $this->fnGetCurrentUser();
        $this->sSysUser = $this->fnGetSysUser();
    }

    function fnPrepareFileInfo(&$aFileInfoData, &$aSplited, &$iErrorCount)
    {
        $aResult = [
            "iErrorCount" => $iErrorCount,
            "iCount" => count($aSplited)
        ];

        $aResult = array_merge($aResult, $aFileInfoData);

        return $aResult;
    }

    function fnPrepareHeaderData($bUpdateData)
    {
        if ($bUpdateData) {
            $this->fnUpdateHeaderData();
        }

        return [
            "sURL" => (string) $this->sURL,
            "sIP" => (string) $this->sIP,
            "sServerIP" => (string) $this->sServerIP,
            "sServerName" => (string) $this->sServerName,
            "bIsCli" => (bool) $this->bIsCli,
            "bIsAjax" => (bool) $this->bIsAjax,
            "aArgv" => isset($_SERVER['argv']) ? (array) $_SERVER['argv'] : [],
            "sUser" => (string) $this->sUser,
            "sSysUser" => (string) $this->sSysUser,
            "sSession" => (string) session_id(),
            "iRequestTimestamp" => (int) $this->iRequestTimestamp,
            "sLang" => (string) $this->sLang,
            "bFinished" => (bool) $this->bFinished,
            "iCode" => (int) $this->iCode,
            "iStartTime" => (int) $this->iStartTime,
            "iStopTime" => (int) $this->iStopTime,
            "iTotalTime" => (int) $this->iStopTime - $this->iStartTime
        ];
    }

    function fnUpdateHeader()
    {
        $resFP = fopen($this->sLogFilePath, 'c');
        fseek($resFP, 0);
        fwrite($resFP, $this->fnPrintHeaderJSON(true)."\n");
        fclose($resFP);
    }

    function fnAppendDataToLog($aData)
    {
        $resFP = fopen($this->sLogFilePath, 'a');
        // fseek($resFP, 0, SEEK_END);
        fwrite($resFP, json_encode($aData, JSON_UNESCAPED_UNICODE)."\n");
        fclose($resFP);
    }

    function fnRegisterShutdownFunction()
    {
        register_shutdown_function([$this, 'fnDebugShutdown']);

        define('DA_RSF', true);
    }

    function fnMakeDirsIfNotExist()
    {
        $sTempPath = $this->sTempPath;
        $sMessagesFilesPath = $this->sMessagesFilesPath;

        if (!is_dir($this->sTempPath)) {
            mkdir($this->sTempPath, 0777);
            chmod($this->sTempPath, 0777);
        }

        if (!is_dir($this->sMessagesFilesPath)) {
            mkdir($this->sMessagesFilesPath, 0777);
            chmod($this->sMessagesFilesPath, 0777);
        }

        return is_dir($this->sTempPath) && is_dir($this->sMessagesFilesPath);
    }

    function fnDebugLog(
        $sType, 
        $sMessage, 
        $aVars=[], 
        $in_iRequestTimestamp=0, 
        $sURL="", 
        $sLang="php",
        $iBacktraceShift = 0
    )
    {
        if (!$this->bIsOnFlag) return;

        if (!$this->iRequestTimestamp) {
            // Пишется заголовок, если запуск в первый раз
            $this->sLang = $sLang;

            if (!$this->fnMakeDirsIfNotExist()) {
                return;
            }

            do {
                if (is_file($this->sLogFilePath)) {
                    sleep(1);
                }

                if ($in_iRequestTimestamp) {
                    $this->iRequestTimestamp = $in_iRequestTimestamp;
                } else {
                    $this->iRequestTimestamp = $this->milliseconds();
                }

                $this->sLogFilePath = "{$this->sMessagesFilesPath}/{$this->iRequestTimestamp}";
            } while(is_file($this->sLogFilePath));

            // error_log($this->fnPrintHeaderJSON(true)."\n", 3, $this->sLogFilePath);
            // $this->fnAppendDataToLog($this->fnPrintHeaderJSON(true));
            $this->fnUpdateHeader();
            chmod($this->sLogFilePath, 0666);

            $this->fnRegisterShutdownFunction();
        }

        if ($this->sSysUser != $this->fnGetSysUser() || $this->bFinished) {
            $this->fnUpdateHeader();
            // __d(DWS_MESSAGE, "\$this->sSysUser = {$this->sSysUser}", [$this->fnPrintHeaderJSON(true)]);
        }

        try {
            
            $aVars = (array) $aVars;

            if ($this->iArraySizeLimit) {
                foreach ($aVars as &$aVar) {
                    if (is_array($aVar) && count($aVar, COUNT_RECURSIVE) > $this->iArraySizeLimit) {
                        $aVar = [];
                    }
                }
                unset($aVar);
            }

            $aVars = $this->fnMaskSecrets($aVars);

            if (!is_file($this->sLogFilePath)) {
                throw new Exception("Файл не найден {$this->sLogFilePath}");
            }

            $aDebugBackTrace = debug_backtrace(DEBUG_BACKTRACE_IGNORE_ARGS, 3);
            $aFrame = isset($aDebugBackTrace[0+$iBacktraceShift]) ? $aDebugBackTrace[0+$iBacktraceShift] : [];
            $aCallerFrame = isset($aDebugBackTrace[1+$iBacktraceShift]) ? $aDebugBackTrace[1+$iBacktraceShift] : [];

            $aData = [
                "sType" => $sType,
                "oBackTrace" => [
                    "file" => isset($aFrame["file"]) ? str_replace(DA_ROOT_DIR, "", $aFrame["file"]) : null,
                    "line" => isset($aFrame["line"]) ? $aFrame["line"] : null,
                    "function" => isset($aCallerFrame["function"]) ? $aCallerFrame["function"] : null,
                ],
                "aBackTrace" => debug_backtrace(DEBUG_BACKTRACE_IGNORE_ARGS),
                "sMessage" => $sMessage,
                "oVars" => (object) $aVars,            
                "iTimestamp" => $this->milliseconds()
            ];

            // error_log(json_encode($aData)."\n", 3, $this->sLogFilePath);
            $this->fnAppendDataToLog($aData);

            /*
            DebugWebsocketClient::send("$sIP:9001", $aData);
            */

            return true;
        } catch(\Exception $oException) {
            return false;
        }
    }

    function fnDebugAddFoundField($sFieldName)
    {
        $this->aFoundFields[] = $sFieldName;
    }

    function fnDebugAddNotFoundField($sFieldName)
    {
        $this->aNotFoundFields[] = $sFieldName;
    }

    function fnStartTimer($sName="")
    {
        if (!DA_FLAG) return;

        if ($sName) {
            $this->aNamedTimers[$sName] = $this->milliseconds();
        } else {
            $this->aTimers[] = $this->milliseconds();
        }
    }

    function fnStopAllTimers()
    {
        if (!DA_FLAG) return;

        foreach ($this->aNamedTimers as $sName => $iStartTime) {
            $this->fnStopTimer($sName);
        }
        foreach ($this->aTimers as $iKey => $iStartTime) {
            $this->fnStopTimer();
        }
    }

    function fnStopTimer($sName="", $iShift=1)
    {
        if (!DA_FLAG) return;

        $sType = DWS_INFO;
        $sTag = "";
        $iStartTime = 0;

        if (!$sName) {
            if (!count($this->aTimers)) {
                return;
            }
            $sName = count($this->aTimers);
            $iStartTime = array_pop($this->aTimers);
        } else {
            if (!count($this->aNamedTimers)) {
                return;
            }
            $iStartTime = $this->aNamedTimers[$sName];
            unset($this->aNamedTimers[$sName]); 
        }

        $sTag = "#$sName";
        $iDiff = (int) $this->milliseconds() - $iStartTime;

        if ($iDiff > 1000) {
            $sType = DWS_WARNING;
        }

        if ($iDiff > 2000) {
            $sType = DWS_ERROR;
        }

        $sMessage = "#timediff $sTag $iDiff мс";

        $this->fnDebugLog($sType, $sMessage, [], 0, "", "php", $iShift);
    }
}

Debugger::fnGetInstance();

function __d($sType, $sMessage, $aVars=[], $iShift=1)
{
    if (!DA_FLAG) return;
    $oD = Debugger::fnGetInstance();
    $oD->fnDebugLog($sType, $sMessage, $aVars, 0, "", "php", $iShift);
}

function fnDebugLog($sType, $sMessage, $aVars=[])
{
    if (!DA_FLAG) return;
    $oD = Debugger::fnGetInstance();
    $oD->fnDebugLog($sType, $sMessage, $aVars, 0, "", "php", 1);
}

function fnDebugAddFoundField($sFieldName)
{
    if (!DA_FLAG) return;
    $oD = Debugger::fnGetInstance();
    $oD->fnDebugAddFoundField($sFieldName);
}

function fnDebugAddNotFoundField($sFieldName)
{
    if (!DA_FLAG) return;
    $oD = Debugger::fnGetInstance();
    $oD->fnDebugAddNotFoundField($sFieldName);
}

function __log($mValue=[])
{
    if (DA_FLAG) {
        $mFirst = is_array($mValue) && count($mValue)==1 ? reset($mValue) : null;
        $sValue = (is_string($mValue) || is_numeric($mValue) ? $mValue : (is_scalar($mFirst) ? $mFirst : '' ));
        $sType = gettype($mValue);
        __d(DWS_MESSAGE, "#inlinelog #{$sType} {$sValue}", $mValue, 2);
    }

    return $mValue;
}

function __start($sName="")
{
    if (!DA_FLAG) return;
    $oD = Debugger::fnGetInstance();
    $oD->fnStartTimer($sName);
}

function __stop($sName="", $iShift=2)
{
    if (!DA_FLAG) return;
    $oD = Debugger::fnGetInstance();
    $oD->fnStopTimer($sName, $iShift);
}

__d(DWS_MESSAGE, '$_SERVER', $_SERVER);
// Сессия и cookie целиком попадают в лог только при bLogSessionAndCookies в конфиге
if (Debugger::fnGetInstance()->bLogSessionAndCookies) {
    __d(DWS_MESSAGE, '$_SESSION', isset($_SESSION) ? $_SESSION : []);
    __d(DWS_MESSAGE, '$_COOKIE', $_COOKIE);
}
__d(DWS_MESSAGE, '$_GET', $_GET);
__d(DWS_MESSAGE, '$_POST', $_POST);
__d(DWS_MESSAGE, '$_FILES', $_FILES);