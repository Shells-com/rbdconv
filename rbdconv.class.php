<?php

// rbdconv class generates rbd export file from raw data
// TODO: add support for qcow2

class rbdconv {
	public $blocksize = 4096;
	public $order = 22; // order 22 (4 MiB objects)
	public $stripe; // 1 << order
	private $out;
	private $buffer = '';
	private $offset = 0;
	private $size;

	public function __construct($out, $size) {
		$this->out = $out;
		$this->size = $size;
		$this->stripe = 1 << $this->order;
		$this->writeHeader();
	}

	public function writeHeader() {
		// https://docs.ceph.com/en/latest/dev/rbd-export/
		fwrite($this->out, "rbd image v2\n");
		$this->writeRecord('O', pack('P', $this->order)); // order
		$this->writeRecord('T', pack('P', 61)); // ??? probably features: layering, exclusive-lock, object-map, fast-diff, deep-flatten
		$this->writeRecord('U', pack('P', $this->stripe)); // stripe unit
		$this->writeRecord('C', pack('P', 1)); // stripe count
		fwrite($this->out, 'E'); // end

		// not documented
		// static const std::string RBD_IMAGE_DIFFS_BANNER_V2 ("rbd image diffs v2\n");
		fwrite($this->out, "rbd image diffs v2\n");
		fwrite($this->out, pack('P', 1));

		// https://github.com/ceph/ceph/blob/master/doc/dev/rbd-diff.rst
		fwrite($this->out, "rbd diff v2\n");
		$this->writeRecord('s', pack('P', $this->size)); // "size"
	}

	public function writeRecord($type, $data) {
		fwrite($this->out, $type.pack('P', strlen($data)).$data);
	}

	public function pushBlock($data) {
		if (strlen($data) != $this->blocksize) {
			// append
			$data .= str_repeat("\0", $this->blocksize - strlen($data));
		}
		$this->buffer .= $data;
		if (strlen($this->buffer) >= $this->stripe) $this->flush();
	}

	public function pushData($data) {
		$this->buffer .= $data;
		if (strlen($this->buffer) < $this->stripe) return true;

		while(strlen($this->buffer) > $this->stripe) {
			$buffer = substr($this->buffer, 0, $this->stripe);
			$offset = $this->offset;
			$this->buffer = (string)substr($this->buffer, $this->stripe);
			$this->offset += strlen($buffer);

			$this->writeBuffer($buffer, $offset);
		}
		return true;
	}

	public function flush() {
		$buffer = $this->buffer;
		$offset = $this->offset;
		if ($buffer == '') return true;

		$this->buffer = '';
		$this->offset += strlen($buffer);

		return $this->writeBuffer($buffer, $offset);
	}

	public function writeBuffer($buffer, $offset) {
		// remove trailing NUL chars
		$buffer = rtrim($buffer, "\0");
		$len = strlen($buffer);
		$padlen = (int)(ceil($len/0x1000))*0x1000;

		if ($len == 0) return true;

		if ($padlen > $len) {
			$buffer .= str_repeat("\0", $padlen-$len);
		}

		// write
		fwrite($this->out, 'w'.pack('PPP', $padlen+8+8, $offset, $padlen));
		fwrite($this->out, $buffer);
		return true;
	}

	public function finalize() {
		fwrite($this->out, 'e'); // "end"
	}

	public function fromRaw($in) {
		$len = 0;
		while(!feof($in)) {
			$data = fread($in, $this->stripe);
			if ($data === false) break;
			$len += strlen($data);
			$this->pushData($data);
		}
		$this->flush();
		$this->finalize();
		return $len;
	}
}
