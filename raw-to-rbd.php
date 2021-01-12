#!/usr/bin/php
<?php

require_once(__DIR__.'/rbdconv.class.php');

// convert RAW image to RBD, output to stdout.

if (count($_SERVER['argv']) >= 2) {
	$in_file = $_SERVER['argv'][1];
	$in = fopen($in_file, 'r');
	if (!$in) die("failed to open file\n");
	$size = filesize($in_file);
} else {
	$in = fopen('php://stdin', 'r');
	$size = 0x200000000; // 8GB - should try to get size by running stat() on stdin?
}

$out = fopen('php://output', 'w');

$obj = new rbdconv($out, $size);
$obj->fromRaw($in);

