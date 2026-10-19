<?php

// The web application of the fixtures, served by php -S with this file as the router.
require '/var/www/lib/autoload.php';

use WappDebugger\Debugger;
use WappDebugger\Level;

Debugger::boot('/var/www/app/dws_debugger.json');

$path = (string) parse_url($_SERVER['REQUEST_URI'], PHP_URL_PATH);
if ($path === '/catalog/item') {
    __d(DWS_INFO, 'Loading the item', ['item_id' => (int) $_GET['id'], 'user' => ['id' => 7, 'api_key' => 'k-123']]);
    __start('db');
    usleep(12000);
    Debugger::instance()->log(Level::DEBUG, 'SELECT * FROM items WHERE id = ?', ['params' => [(int) $_GET['id']]], ['sql']);
    __stop('db');
    // A real E_WARNING (notices are outside the default error_reporting of PHP 7.4 without php.ini)
    $prices = fopen('/var/www/app/tmp/prices.csv', 'r');
    // The page hands its request id to its scripts: they send it in X-Wapp-Debugger-Parent
    echo Debugger::instance()->requestId();

    return;
}
if ($path === '/api/cart/add') {
    __d(DWS_MESSAGE, 'Added to the cart', $_POST);
    http_response_code(201);
    echo '{"ok":true}';

    return;
}
http_response_code(404);
